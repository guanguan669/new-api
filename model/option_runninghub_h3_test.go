package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestLoadOptionValuesLoadsStandaloneRunningHubH3PricePlan(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	originalH3Prices := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(originalH3Prices))
	})

	loadOptionValues([]*Option{
		{Key: ratio_setting.RunningHubH3GroupPriceOptionKey, Value: `{"private-plan":{"price_768p":0.1,"price_2k":0.2}}`},
	})

	price, ok := ratio_setting.GetRunningHubH3GroupPrice("private-plan")
	require.True(t, ok)
	assert.Equal(t, ratio_setting.RunningHubH3GroupPrice{Price768P: 0.1, Price2K: 0.2}, price)
}

func TestLoadOptionValuesLoadsNamespacedStandaloneRunningHubH3PricePlan(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	originalH3Prices := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(originalH3Prices))
	})

	loadOptionValues([]*Option{
		{Key: "group_ratio_setting.runninghub_h3_group_price", Value: `{"namespaced-plan":{"price_768p":0.3,"price_2k":0.4}}`},
	})

	price, ok := ratio_setting.GetRunningHubH3GroupPrice("namespaced-plan")
	require.True(t, ok)
	assert.Equal(t, ratio_setting.RunningHubH3GroupPrice{Price768P: 0.3, Price2K: 0.4}, price)
	common.OptionMapRWMutex.Lock()
	assert.Equal(t, `{"namespaced-plan":{"price_768p":0.3,"price_2k":0.4}}`, common.OptionMap[ratio_setting.RunningHubH3GroupPriceOptionKey])
	_, hasNamespacedKey := common.OptionMap[runningHubH3NamespacedPriceOptionKey]
	common.OptionMapRWMutex.Unlock()
	assert.False(t, hasNamespacedKey)
}

func TestLoadOptionValuesCanonicalRunningHubH3PriceWins(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	originalH3Prices := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(originalH3Prices))
	})

	loadOptionValues([]*Option{
		{Key: runningHubH3NamespacedPriceOptionKey, Value: `{"legacy":{"price_768p":0.9,"price_2k":1.8}}`},
		{Key: ratio_setting.RunningHubH3GroupPriceOptionKey, Value: `{"canonical":{"price_768p":0.1,"price_2k":0.2}}`},
	})

	_, legacyExists := ratio_setting.GetRunningHubH3GroupPrice("legacy")
	canonical, canonicalExists := ratio_setting.GetRunningHubH3GroupPrice("canonical")
	assert.False(t, legacyExists)
	require.True(t, canonicalExists)
	assert.Equal(t, ratio_setting.RunningHubH3GroupPrice{Price768P: 0.1, Price2K: 0.2}, canonical)
}

func TestUpdateNamespacedRunningHubH3PriceConvergesToCanonicalOption(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Exec("DELETE FROM options").Error)
	t.Cleanup(func() {
		_ = DB.Exec("DELETE FROM options").Error
	})

	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	originalH3Prices := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(originalH3Prices))
	})
	require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(`{"old":{"price_768p":0.1,"price_2k":0.2}}`))
	require.NoError(t, DB.Create(&Option{Key: ratio_setting.RunningHubH3GroupPriceOptionKey, Value: `{"old":{"price_768p":0.1,"price_2k":0.2}}`}).Error)
	require.NoError(t, DB.Create(&Option{Key: runningHubH3NamespacedPriceOptionKey, Value: `{"legacy":{"price_768p":0.4,"price_2k":0.8}}`}).Error)

	newValue := `{"new":{"price_768p":0.3,"price_2k":0.6}}`
	require.NoError(t, UpdateOption(runningHubH3NamespacedPriceOptionKey, newValue))

	var canonical Option
	require.NoError(t, DB.First(&canonical, "key = ?", ratio_setting.RunningHubH3GroupPriceOptionKey).Error)
	assert.Equal(t, newValue, canonical.Value)
	assert.ErrorIs(t, DB.First(&Option{}, "key = ?", runningHubH3NamespacedPriceOptionKey).Error, gorm.ErrRecordNotFound)
	price, ok := ratio_setting.GetRunningHubH3GroupPrice("new")
	require.True(t, ok)
	assert.Equal(t, ratio_setting.RunningHubH3GroupPrice{Price768P: 0.3, Price2K: 0.6}, price)
}

func TestRunningHubH3PriceGroupRemovalUsesMySQLCurrentRead(t *testing.T) {
	sqlDB, err := DB.DB()
	require.NoError(t, err)
	mysqlDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true})
	require.NoError(t, err)

	users := make([]RunningHubH3PriceGroupUser, 0)
	statement := runningHubH3PriceGroupUsersForRemovalQuery(mysqlDB).
		Find(&users).Statement
	assert.Contains(t, strings.ToUpper(statement.SQL.String()), "FOR UPDATE")
}

func TestLockRunningHubH3PriceGroupsAppliesLegacySameNameBinding(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	originalGroups := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"route-pro":1}`))
	require.NoError(t, DB.Create(&Option{
		Key:   ratio_setting.RunningHubH3GroupPriceOptionKey,
		Value: `{"route-pro":{"price_768p":0.3,"price_2k":0.4}}`,
	}).Error)

	configured, err := lockRunningHubH3PriceGroups(DB)
	require.NoError(t, err)
	assert.Equal(t, "route-pro", configured["route-pro"].BoundGroup)
}

func TestValidateOptionValueValidatesNamespacedRunningHubH3Prices(t *testing.T) {
	assert.NoError(t, validateOptionValue("group_ratio_setting.runninghub_h3_group_price", `{"private-plan":{"price_768p":0,"price_2k":0}}`))
	assert.Error(t, validateOptionValue("group_ratio_setting.runninghub_h3_group_price", `{" private ":{"price_768p":0,"price_2k":0}}`))
}

func TestValidateOptionValueRejectsRemovingAssignedRunningHubH3PriceTier(t *testing.T) {
	truncateTables(t)
	originalH3Prices := ratio_setting.RunningHubH3GroupPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(originalH3Prices))
	})
	require.NoError(t, ratio_setting.UpdateRunningHubH3GroupPriceByJSONString(
		`{"basic":{"price_768p":0.1,"price_2k":0.2},"pro":{"price_768p":0.3,"price_2k":0.4}}`,
	))

	user := User{
		Username: "h3-tier-member", Password: "password123", DisplayName: "H3 Tier Member",
		Role: common.RoleCommonUser, Status: common.UserStatusDisabled, Group: "shared-h3", AffCode: "h3-tier-member-aff",
	}
	user.SetSetting(dto.UserSetting{RunningHubH3PriceGroup: "basic", Language: "zh"})
	require.NoError(t, DB.Create(&user).Error)

	err := validateOptionValue(
		ratio_setting.RunningHubH3GroupPriceOptionKey,
		`{"pro":{"price_768p":0.3,"price_2k":0.4}}`,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "basic (1 users)")

	assert.NoError(t, validateOptionValue(
		ratio_setting.RunningHubH3GroupPriceOptionKey,
		`{"basic":{"price_768p":0.2,"price_2k":0.5},"pro":{"price_768p":0.3,"price_2k":0.4}}`,
	))

	require.NoError(t, DB.Delete(&user).Error)
	assert.NoError(t, validateOptionValue(
		ratio_setting.RunningHubH3GroupPriceOptionKey,
		`{"pro":{"price_768p":0.3,"price_2k":0.4}}`,
	))
}
