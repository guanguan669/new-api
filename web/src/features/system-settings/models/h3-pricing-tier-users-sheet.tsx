/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Search, UserMinus, UserPlus, UsersRound } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  SideDrawerSection,
  SideDrawerSectionHeader,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  getH3PriceGroupUsers,
  updateUserH3PriceGroup,
} from '@/features/users/api'
import type { H3PricingTierUser } from '@/features/users/types'
import { useDebounce } from '@/hooks'

const PAGE_SIZE = 20

type H3PricingTierUsersSheetProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  tier: string
  price768P: number
  price2K: number
}

type AssignmentMutation = {
  user: AssignmentUser
  tier: string
  previousTier: string
}

type AssignmentUser = H3PricingTierUser

function getUserDisplayName(user: AssignmentUser) {
  return user.display_name?.trim() || user.username
}

export function H3PricingTierUsersSheet({
  open,
  onOpenChange,
  tier,
  price768P,
  price2K,
}: H3PricingTierUsersSheetProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [memberPage, setMemberPage] = useState(1)
  const [searchValue, setSearchValue] = useState('')
  const debouncedSearchValue = useDebounce(searchValue.trim(), 350)

  useEffect(() => {
    if (!open) return
    setMemberPage(1)
    setSearchValue('')
  }, [open, tier])

  const membersQuery = useQuery({
    queryKey: ['h3-price-group-users', tier, memberPage],
    queryFn: () =>
      getH3PriceGroupUsers({
        group: tier,
        p: memberPage,
        page_size: PAGE_SIZE,
      }),
    enabled: open && Boolean(tier),
  })

  const candidatesQuery = useQuery({
    queryKey: ['h3-pricing-tier-user-search', tier, debouncedSearchValue],
    queryFn: () =>
      getH3PriceGroupUsers({
        group: tier,
        keyword: debouncedSearchValue,
        scope: 'available',
        status: '1',
        p: 1,
        page_size: PAGE_SIZE,
      }),
    enabled: open && debouncedSearchValue.length > 0,
  })

  const memberItems = membersQuery.data?.data?.items
  const members = useMemo(() => memberItems ?? [], [memberItems])
  const memberTotal = membersQuery.data?.data?.total ?? 0
  const memberPageSize = membersQuery.data?.data?.page_size ?? PAGE_SIZE
  const memberPageCount = Math.max(1, Math.ceil(memberTotal / memberPageSize))
  const candidates = candidatesQuery.data?.data?.items ?? []

  useEffect(() => {
    if (memberPage > memberPageCount) setMemberPage(memberPageCount)
  }, [memberPage, memberPageCount])

  const assignmentMutation = useMutation({
    mutationFn: async ({ user, tier: nextTier }: AssignmentMutation) => {
      const result = await updateUserH3PriceGroup(user.id, nextTier)
      if (!result.success) {
        throw new Error(result.message || t('Failed to update H3 pricing tier'))
      }
      return result
    },
    onSuccess: async (_, variables) => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['h3-price-group-users'],
        }),
        queryClient.invalidateQueries({ queryKey: ['h3-price-groups'] }),
        queryClient.invalidateQueries({ queryKey: ['users'] }),
        queryClient.invalidateQueries({
          queryKey: ['h3-pricing-tier-user-search'],
        }),
      ])

      if (!variables.tier) {
        toast.success(
          t('Removed {{user}} from H3 pricing tier {{tier}}.', {
            user: getUserDisplayName(variables.user),
            tier,
          })
        )
        return
      }

      toast.success(
        variables.previousTier
          ? t('Moved {{user}} from {{from}} to {{to}}.', {
              user: getUserDisplayName(variables.user),
              from: variables.previousTier,
              to: variables.tier,
            })
          : t('Added {{user}} to H3 pricing tier {{tier}}.', {
              user: getUserDisplayName(variables.user),
              tier: variables.tier,
            })
      )
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to update H3 pricing tier'))
    },
  })

  const pendingUserId = assignmentMutation.variables?.user.id

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle className='pr-10'>
            {t('Users in H3 pricing tier {{tier}}', { tier })}
          </SheetTitle>
          <SheetDescription>
            {t('768P: ¥{{price768P}}/sec · 2K: ¥{{price2K}}/sec', {
              price768P,
              price2K,
            })}
          </SheetDescription>
        </SheetHeader>

        <div className={sideDrawerFormClassName()}>
          <SideDrawerSection>
            <SideDrawerSectionHeader
              icon={<UserPlus className='h-4 w-4' />}
              title={t('Add or move user')}
              description={t(
                'Search by user ID, username, display name, or email. The user keeps the same normal routing group.'
              )}
            />

            <div className='relative'>
              <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
              <Input
                value={searchValue}
                onChange={(event) => setSearchValue(event.target.value)}
                placeholder={t('Search users')}
                aria-label={t('Search users')}
                className='pl-9'
              />
            </div>

            {debouncedSearchValue ? (
              <div className='border-border/70 divide-border/60 divide-y border-y'>
                {candidatesQuery.isLoading && (
                  <div className='text-muted-foreground flex h-20 items-center justify-center gap-2 text-sm'>
                    <Loader2 className='h-4 w-4 animate-spin' />
                    {t('Loading...')}
                  </div>
                )}
                {!candidatesQuery.isLoading && candidates.length === 0 && (
                  <div className='text-muted-foreground flex h-20 items-center justify-center text-sm'>
                    {t('No users found.')}
                  </div>
                )}
                {!candidatesQuery.isLoading &&
                  candidates.length > 0 &&
                  candidates.map((user) => {
                    const currentTier = user.h3_price_group
                    const alreadyAssigned = currentTier === tier
                    const isPending =
                      assignmentMutation.isPending && pendingUserId === user.id
                    let actionLabel = t('Add')
                    if (alreadyAssigned) actionLabel = t('Assigned')
                    else if (currentTier) actionLabel = t('Move here')

                    return (
                      <div
                        key={user.id}
                        className='flex min-h-16 items-center gap-3 py-2'
                      >
                        <div className='min-w-0 flex-1'>
                          <p className='truncate text-sm font-medium'>
                            {getUserDisplayName(user)}
                          </p>
                          <p className='text-muted-foreground truncate text-xs'>
                            #{user.id} · {user.username}
                            {user.email ? ` · ${user.email}` : ''}
                          </p>
                          <p className='text-muted-foreground mt-0.5 truncate text-xs'>
                            {currentTier
                              ? t('Current H3 pricing tier: {{tier}}', {
                                  tier: currentTier,
                                })
                              : t('No H3 pricing tier assigned')}
                            {' · '}
                            {t('Normal group: {{group}}', {
                              group: user.group,
                            })}
                          </p>
                        </div>
                        <Button
                          type='button'
                          size='sm'
                          variant={alreadyAssigned ? 'outline' : 'default'}
                          disabled={alreadyAssigned || isPending}
                          onClick={() =>
                            assignmentMutation.mutate({
                              user,
                              tier,
                              previousTier: currentTier,
                            })
                          }
                        >
                          {isPending ? (
                            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                          ) : (
                            <UserPlus className='mr-2 h-4 w-4' />
                          )}
                          {actionLabel}
                        </Button>
                      </div>
                    )
                  })}
              </div>
            ) : (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Enter a keyword to find users to add to this pricing tier.'
                )}
              </p>
            )}
          </SideDrawerSection>

          <SideDrawerSection>
            <SideDrawerSectionHeader
              icon={<UsersRound className='h-4 w-4' />}
              title={t('Assigned users ({{count}})', { count: memberTotal })}
              description={t(
                'Removing a user clears only the H3 pricing tier and does not change the normal routing group.'
              )}
            />

            {membersQuery.isLoading && (
              <div className='text-muted-foreground flex h-24 items-center justify-center gap-2 text-sm'>
                <Loader2 className='h-4 w-4 animate-spin' />
                {t('Loading...')}
              </div>
            )}
            {!membersQuery.isLoading && members.length === 0 && (
              <div className='text-muted-foreground flex h-24 items-center justify-center border-y text-sm'>
                {t('No users use this H3 pricing tier.')}
              </div>
            )}
            {!membersQuery.isLoading && members.length > 0 && (
              <div className='border-border/70 divide-border/60 divide-y border-y'>
                {members.map((user) => {
                  const isPending =
                    assignmentMutation.isPending && pendingUserId === user.id
                  return (
                    <div
                      key={user.id}
                      className='flex min-h-14 items-center gap-3 py-2'
                    >
                      <div className='min-w-0 flex-1'>
                        <p className='truncate text-sm font-medium'>
                          {getUserDisplayName(user)}
                        </p>
                        <p className='text-muted-foreground truncate text-xs'>
                          #{user.id} · {user.username} ·{' '}
                          {t('Normal group: {{group}}', { group: user.group })}
                        </p>
                      </div>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              type='button'
                              size='icon-sm'
                              variant='ghost'
                              disabled={isPending}
                              aria-label={t('Remove user from H3 pricing tier')}
                              onClick={() =>
                                assignmentMutation.mutate({
                                  user,
                                  tier: '',
                                  previousTier: tier,
                                })
                              }
                            />
                          }
                        >
                          {isPending ? (
                            <Loader2 className='h-4 w-4 animate-spin' />
                          ) : (
                            <UserMinus className='h-4 w-4' />
                          )}
                        </TooltipTrigger>
                        <TooltipContent>
                          {t('Remove user from H3 pricing tier')}
                        </TooltipContent>
                      </Tooltip>
                    </div>
                  )
                })}
              </div>
            )}

            {memberPageCount > 1 && (
              <div className='flex items-center justify-between gap-3'>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={memberPage <= 1 || membersQuery.isFetching}
                  onClick={() => setMemberPage((page) => Math.max(1, page - 1))}
                >
                  {t('Previous')}
                </Button>
                <span className='text-muted-foreground text-xs'>
                  {t('Page {{page}} of {{pages}}', {
                    page: memberPage,
                    pages: memberPageCount,
                  })}
                </span>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={
                    memberPage >= memberPageCount || membersQuery.isFetching
                  }
                  onClick={() =>
                    setMemberPage((page) => Math.min(memberPageCount, page + 1))
                  }
                >
                  {t('Next')}
                </Button>
              </div>
            )}
          </SideDrawerSection>
        </div>
      </SheetContent>
    </Sheet>
  )
}
