package service

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"

	"gorm.io/gorm"
)

var creditCodePattern = regexp.MustCompile(`^[0-9A-Z]{18}$`)

var enterpriseIndustries = []string{
	"农、林、牧、渔业", "采矿业", "制造业", "电力、热力、燃气及水生产和供应业", "建筑业",
	"批发和零售业", "交通运输、仓储和邮政业", "住宿和餐饮业", "信息传输、软件和信息技术服务业",
	"金融业", "房地产业", "租赁和商务服务业", "科学研究和技术服务业", "水利、环境和公共设施管理业",
	"居民服务、修理和其他服务业", "教育", "卫生和社会工作", "文化、体育和娱乐业",
	"公共管理、社会保障和社会组织", "国际组织",
}

var enterpriseScales = []string{"大型", "中型", "小型", "微型"}

func containsOption(options []string, value string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func validateCreditCode(value string) error {
	if utf8.RuneCountInString(value) != 18 || !creditCodePattern.MatchString(value) {
		return errcode.ErrInvalidParams.WithMsg("统一社会信用代码必须为18位数字或大写英文字母，不得包含小写字母、符号或空格")
	}
	return nil
}

func validateEnterpriseIndustry(value string) error {
	if !containsOption(enterpriseIndustries, value) {
		return errcode.ErrInvalidParams.WithMsg("所属行业必须从系统提供的选项中选择")
	}
	return nil
}

func validateEnterpriseScale(value string) error {
	if !containsOption(enterpriseScales, value) {
		return errcode.ErrInvalidParams.WithMsg("企业规模必须从系统提供的选项中选择")
	}
	return nil
}

func validateEnterpriseIdentity(db *gorm.DB, name, creditCode string, excludeID uint) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errcode.ErrInvalidParams.WithMsg("企业名称不能为空")
	}
	if err := validateCreditCode(creditCode); err != nil {
		return err
	}
	var count int64
	query := db.Table("enterprises").Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if query.Count(&count).Error != nil {
		return errcode.ErrInternal
	}
	if count > 0 {
		return errcode.ErrDuplicate.WithMsg("企业名称已存在")
	}
	query = db.Table("enterprises").Where("credit_code = ?", creditCode)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if query.Count(&count).Error != nil {
		return errcode.ErrInternal
	}
	if count > 0 {
		return errcode.ErrDuplicate.WithMsg("统一社会信用代码已存在")
	}
	return nil
}

func changeValue(values model.JSONMap) (string, error) {
	value, ok := values["value"].(string)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return "", errcode.ErrInvalidParams.WithMsg("变更后的内容不能为空")
	}
	return value, nil
}

func (s *EnterpriseService) validateChangeValue(ent *model.Enterprise, changeType string, values model.JSONMap) error {
	return validateChangeValueWithDB(s.db, ent, changeType, values)
}

func validateChangeValueWithDB(db *gorm.DB, ent *model.Enterprise, changeType string, values model.JSONMap) error {
	if changeType == "入孵协议文件" {
		return nil
	}
	value, err := changeValue(values)
	if err != nil {
		return err
	}
	switch changeType {
	case "企业名称":
		return validateEnterpriseIdentity(db, value, ent.CreditCode, ent.ID)
	case "统一社会信用代码":
		return validateEnterpriseIdentity(db, ent.Name, value, ent.ID)
	case "所属行业":
		return validateEnterpriseIndustry(value)
	case "企业规模":
		return validateEnterpriseScale(value)
	}
	return nil
}
