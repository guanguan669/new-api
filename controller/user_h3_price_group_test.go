package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type h3TestResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type h3TestPage struct {
	Page     int                              `json:"page"`
	PageSize int                              `json:"page_size"`
	Total    int                              `json:"total"`
	Items    []runningHubH3PriceGroupUserItem `json:"items"`
}

func setupH3PriceGroupTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	originalPrices := ratio_setting.RunningHubH3GroupPrice2JSONString()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	configuredPrices := `{"basic":{"price_768p":0.1,"price_2k":0.2},"pro":{"price_768p":0.3,"price_2k":0.4}}`
	require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(configuredPrices))

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.Option{}))
	require.NoError(t, db.Create(&model.Option{
		Key:   ratio_setting.RunningHubH3GroupPriceOptionKey,
		Value: configuredPrices,
	}).Error)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[ratio_setting.RunningHubH3GroupPriceOptionKey] = configuredPrices
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(originalPrices))
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createH3TestUser(t *testing.T, db *gorm.DB, username string, role, status int, routingGroup, priceGroup string) model.User {
	t.Helper()
	user := model.User{
		Username: username, Password: "password", DisplayName: "Display " + username,
		Email: username + "@example.com", Role: role, Status: status, Group: routingGroup, AffCode: "aff-" + username,
	}
	user.SetSetting(dto.UserSetting{RunningHubH3PriceGroup: priceGroup, Language: "zh"})
	require.NoError(t, db.Create(&user).Error)
	return user
}

func performH3Request(t *testing.T, method, target, body string, role int, handler gin.HandlerFunc) h3TestResponse {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Handle(method, "/api/user/h3-price-groups/:group/users", func(c *gin.Context) {
		c.Set("id", 9999)
		c.Set("role", role)
		c.Set("username", "root-operator")
		handler(c)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response h3TestResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestGetRunningHubH3PriceGroupsCountsAssignedNonDeletedUsers(t *testing.T) {
	db := setupH3PriceGroupTest(t)
	createH3TestUser(t, db, "active-basic", common.RoleCommonUser, common.UserStatusEnabled, "route-a", "basic")
	createH3TestUser(t, db, "disabled-basic", common.RoleCommonUser, common.UserStatusDisabled, "route-b", "basic")
	createH3TestUser(t, db, "admin-basic-count", common.RoleAdminUser, common.UserStatusEnabled, "route-admin", "basic")
	deleted := createH3TestUser(t, db, "deleted-pro", common.RoleCommonUser, common.UserStatusEnabled, "route-c", "pro")
	require.NoError(t, db.Delete(&deleted).Error)
	createH3TestUser(t, db, "active-pro", common.RoleCommonUser, common.UserStatusEnabled, "route-d", "pro")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/user/h3-price-groups", nil)
	c.Set("role", common.RoleRootUser)
	GetRunningHubH3PriceGroups(c)
	var response struct {
		Success bool                         `json:"success"`
		Data    []runningHubH3PriceGroupItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 2)
	assert.Equal(t, "basic", response.Data[0].Group)
	assert.Equal(t, 3, response.Data[0].UserCount)
	assert.Equal(t, "pro", response.Data[1].Group)
	assert.Equal(t, 1, response.Data[1].UserCount)

	adminRecorder := httptest.NewRecorder()
	adminContext, _ := gin.CreateTestContext(adminRecorder)
	adminContext.Request = httptest.NewRequest(http.MethodGet, "/api/user/h3-price-groups", nil)
	adminContext.Set("role", common.RoleAdminUser)
	GetRunningHubH3PriceGroups(adminContext)
	require.NoError(t, json.Unmarshal(adminRecorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 2)
	assert.Equal(t, 2, response.Data[0].UserCount)
	assert.Equal(t, 1, response.Data[1].UserCount)
}

func TestGetRunningHubH3PriceGroupUsersFiltersPaginatesAndEnforcesRole(t *testing.T) {
	db := setupH3PriceGroupTest(t)
	commonOne := createH3TestUser(t, db, "alice", common.RoleCommonUser, common.UserStatusEnabled, "route-one", "basic")
	commonTwo := createH3TestUser(t, db, "bob", common.RoleCommonUser, common.UserStatusDisabled, "route-two", "basic")
	createH3TestUser(t, db, "admin-basic", common.RoleAdminUser, common.UserStatusEnabled, "route-admin", "basic")
	createH3TestUser(t, db, "other-tier", common.RoleCommonUser, common.UserStatusEnabled, "route-pro", "pro")
	deleted := createH3TestUser(t, db, "deleted-basic", common.RoleCommonUser, common.UserStatusEnabled, "route-deleted", "basic")
	require.NoError(t, db.Delete(&deleted).Error)

	response := performH3Request(t, http.MethodGet, "/api/user/h3-price-groups/basic/users?p=1&page_size=1", "", common.RoleAdminUser, GetRunningHubH3PriceGroupUsers)
	require.True(t, response.Success)
	var page h3TestPage
	require.NoError(t, json.Unmarshal(response.Data, &page))
	assert.Equal(t, 2, page.Total)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 1, page.PageSize)
	require.Len(t, page.Items, 1)
	assert.Equal(t, commonTwo.Id, page.Items[0].Id)
	assert.Equal(t, "route-two", page.Items[0].Group)
	assert.Equal(t, "basic", page.Items[0].H3PriceGroup)

	response = performH3Request(t, http.MethodGet, "/api/user/h3-price-groups/basic/users?keyword=ALICE", "", common.RoleAdminUser, GetRunningHubH3PriceGroupUsers)
	require.NoError(t, json.Unmarshal(response.Data, &page))
	assert.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	assert.Equal(t, commonOne.Id, page.Items[0].Id)

	response = performH3Request(t, http.MethodGet, "/api/user/h3-price-groups/basic/users", "", common.RoleRootUser, GetRunningHubH3PriceGroupUsers)
	require.NoError(t, json.Unmarshal(response.Data, &page))
	assert.Equal(t, 3, page.Total)

	response = performH3Request(t, http.MethodGet, "/api/user/h3-price-groups/basic/users?scope=available&status=1&keyword=other", "", common.RoleAdminUser, GetRunningHubH3PriceGroupUsers)
	require.NoError(t, json.Unmarshal(response.Data, &page))
	assert.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "other-tier", page.Items[0].Username)
	assert.Equal(t, "pro", page.Items[0].H3PriceGroup)

	response = performH3Request(t, http.MethodGet, "/api/user/h3-price-groups/basic/users?scope=available&keyword=admin-basic", "", common.RoleAdminUser, GetRunningHubH3PriceGroupUsers)
	require.NoError(t, json.Unmarshal(response.Data, &page))
	assert.Zero(t, page.Total)
}

func TestGetRunningHubH3PriceGroupUsersRejectsUnknownTier(t *testing.T) {
	setupH3PriceGroupTest(t)
	response := performH3Request(t, http.MethodGet, "/api/user/h3-price-groups/missing/users", "", common.RoleRootUser, GetRunningHubH3PriceGroupUsers)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "not configured")
}

func TestGetRunningHubH3PriceGroupUsersRejectsInvalidScope(t *testing.T) {
	setupH3PriceGroupTest(t)
	response := performH3Request(t, http.MethodGet, "/api/user/h3-price-groups/basic/users?scope=invalid", "", common.RoleRootUser, GetRunningHubH3PriceGroupUsers)
	assert.False(t, response.Success)
}

func TestUpdateUserRunningHubH3PriceGroupPreservesRoutingGroupAndSettings(t *testing.T) {
	db := setupH3PriceGroupTest(t)
	user := model.User{
		Username: "assigned-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "routing-premium", AffCode: "aff-assigned-user",
	}
	user.SetSetting(dto.UserSetting{Language: "en", BillingPreference: "wallet", RunningHubH3PriceGroup: "basic"})
	require.NoError(t, db.Create(&user).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/user/:id/h3-price-group", func(c *gin.Context) {
		c.Set("id", 9999)
		c.Set("role", common.RoleRootUser)
		c.Set("username", "root-operator")
		UpdateUserRunningHubH3PriceGroup(c)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/user/%d/h3-price-group", user.Id), strings.NewReader(`{"group":"pro"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, "routing-premium", updated.Group)
	setting := updated.GetSetting()
	assert.Equal(t, "pro", setting.RunningHubH3PriceGroup)
	assert.Equal(t, "en", setting.Language)
	assert.Equal(t, "wallet", setting.BillingPreference)
}

func TestOrdinaryUserSettingsKeepRunningHubH3PriceTier(t *testing.T) {
	t.Run("notification settings", func(t *testing.T) {
		db := setupH3PriceGroupTest(t)
		user := createH3TestUser(t, db, "notify-user", common.RoleCommonUser, common.UserStatusEnabled, "shared-route", "basic")
		require.NoError(t, model.MutateUserSetting(user.Id, func(setting *dto.UserSetting) error {
			setting.Language = "zh"
			setting.SidebarModules = `{"console":true}`
			setting.BillingPreference = "wallet"
			setting.NotifyType = dto.NotifyTypeWebhook
			setting.WebhookUrl = "https://old.example.com/hook"
			return nil
		}))

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PUT("/api/user/setting", func(c *gin.Context) {
			c.Set("id", user.Id)
			c.Set("role", common.RoleCommonUser)
			UpdateUserSetting(c)
		})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/api/user/setting", strings.NewReader(`{
			"notify_type":"email",
			"quota_warning_threshold":10,
			"notification_email":"notify@example.com",
			"accept_unset_model_ratio_model":true,
			"record_ip_log":true
		}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		assert.Contains(t, recorder.Body.String(), `"success":true`)

		var updated model.User
		require.NoError(t, db.First(&updated, user.Id).Error)
		setting := updated.GetSetting()
		assert.Equal(t, "basic", setting.RunningHubH3PriceGroup)
		assert.Equal(t, "zh", setting.Language)
		assert.Equal(t, `{"console":true}`, setting.SidebarModules)
		assert.Equal(t, "wallet", setting.BillingPreference)
		assert.Equal(t, dto.NotifyTypeEmail, setting.NotifyType)
		assert.Equal(t, "notify@example.com", setting.NotificationEmail)
		assert.Empty(t, setting.WebhookUrl)
	})

	t.Run("language and sidebar settings", func(t *testing.T) {
		db := setupH3PriceGroupTest(t)
		user := createH3TestUser(t, db, "self-setting-user", common.RoleCommonUser, common.UserStatusEnabled, "shared-route", "pro")

		for _, body := range []string{`{"language":"en"}`, `{"sidebar_modules":"custom-sidebar"}`} {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.PUT("/api/user/self", func(c *gin.Context) {
				c.Set("id", user.Id)
				UpdateSelf(c)
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/api/user/self", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			assert.Contains(t, recorder.Body.String(), `"success":true`)
		}

		var updated model.User
		require.NoError(t, db.First(&updated, user.Id).Error)
		setting := updated.GetSetting()
		assert.Equal(t, "pro", setting.RunningHubH3PriceGroup)
		assert.Equal(t, "en", setting.Language)
		assert.Equal(t, "custom-sidebar", setting.SidebarModules)
	})

	t.Run("subscription preference", func(t *testing.T) {
		db := setupH3PriceGroupTest(t)
		user := createH3TestUser(t, db, "subscription-user", common.RoleCommonUser, common.UserStatusEnabled, "shared-route", "basic")
		require.NoError(t, model.MutateUserSetting(user.Id, func(setting *dto.UserSetting) error {
			setting.NotifyType = dto.NotifyTypeEmail
			setting.NotificationEmail = "billing@example.com"
			return nil
		}))

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PUT("/api/subscription/preference", func(c *gin.Context) {
			c.Set("id", user.Id)
			UpdateSubscriptionPreference(c)
		})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/api/subscription/preference", strings.NewReader(`{"billing_preference":"subscription"}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		assert.Contains(t, recorder.Body.String(), `"success":true`)

		var updated model.User
		require.NoError(t, db.First(&updated, user.Id).Error)
		setting := updated.GetSetting()
		assert.Equal(t, "basic", setting.RunningHubH3PriceGroup)
		assert.Equal(t, "subscription_first", setting.BillingPreference)
		assert.Equal(t, dto.NotifyTypeEmail, setting.NotifyType)
		assert.Equal(t, "billing@example.com", setting.NotificationEmail)
	})
}

func TestRunningHubH3PriceTierAssignmentAndRemovalSerialize(t *testing.T) {
	t.Run("assignment first blocks removal", func(t *testing.T) {
		db := setupH3PriceGroupTest(t)
		user := createH3TestUser(t, db, "assign-first", common.RoleCommonUser, common.UserStatusEnabled, "shared-route", "")

		response := performH3AssignmentRequest(t, user.Id, `{"group":"basic"}`)
		require.True(t, response.Success)

		err := model.UpdateOption(
			ratio_setting.RunningHubH3GroupPriceOptionKey,
			`{"pro":{"price_768p":0.3,"price_2k":0.4}}`,
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "basic (1 users)")
	})

	t.Run("removal first blocks assignment", func(t *testing.T) {
		db := setupH3PriceGroupTest(t)
		user := createH3TestUser(t, db, "remove-first", common.RoleCommonUser, common.UserStatusEnabled, "shared-route", "")

		require.NoError(t, model.UpdateOption(
			ratio_setting.RunningHubH3GroupPriceOptionKey,
			`{"pro":{"price_768p":0.3,"price_2k":0.4}}`,
		))
		response := performH3AssignmentRequest(t, user.Id, `{"group":"basic"}`)
		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "not configured")

		var updated model.User
		require.NoError(t, db.First(&updated, user.Id).Error)
		assert.Equal(t, "shared-route", updated.Group)
		assert.Empty(t, updated.GetSetting().RunningHubH3PriceGroup)
	})
}

func TestRunningHubH3PriceTierConcurrentAssignmentAndRemovalStayConsistent(t *testing.T) {
	db := setupH3PriceGroupTest(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	user := createH3TestUser(t, db, "concurrent-tier-user", common.RoleCommonUser, common.UserStatusEnabled, "shared-route", "")

	start := make(chan struct{})
	var wg sync.WaitGroup
	var assignmentResponse h3TestResponse
	var assignmentErr error
	var removalErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		assignmentResponse, assignmentErr = executeH3AssignmentRequest(user.Id, `{"group":"basic"}`)
	}()
	go func() {
		defer wg.Done()
		<-start
		removalErr = model.UpdateOption(
			ratio_setting.RunningHubH3GroupPriceOptionKey,
			`{"pro":{"price_768p":0.3,"price_2k":0.4}}`,
		)
	}()
	close(start)
	wg.Wait()

	require.NoError(t, assignmentErr)
	removalSucceeded := removalErr == nil
	assert.NotEqual(t, assignmentResponse.Success, removalSucceeded)

	var option model.Option
	require.NoError(t, db.First(&option, "key = ?", ratio_setting.RunningHubH3GroupPriceOptionKey).Error)
	configured := make(map[string]ratio_setting.RunningHubH3GroupPrice)
	require.NoError(t, json.Unmarshal([]byte(option.Value), &configured))
	_, tierExists := configured["basic"]

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assigned := updated.GetSetting().RunningHubH3PriceGroup == "basic"
	assert.Equal(t, tierExists, assigned)
}

func performH3AssignmentRequest(t *testing.T, userId int, body string) h3TestResponse {
	t.Helper()
	response, err := executeH3AssignmentRequest(userId, body)
	require.NoError(t, err)
	return response
}

func executeH3AssignmentRequest(userId int, body string) (h3TestResponse, error) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/user/:id/h3-price-group", func(c *gin.Context) {
		c.Set("id", 9999)
		c.Set("role", common.RoleRootUser)
		c.Set("username", "root-operator")
		UpdateUserRunningHubH3PriceGroup(c)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/user/%d/h3-price-group", userId), strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	var response h3TestResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	return response, err
}
