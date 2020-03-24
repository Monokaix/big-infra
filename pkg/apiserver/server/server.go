package server

import (
	"big-infra/pkg/apiserver/common"
	"big-infra/pkg/model"
	"github.com/jinzhu/gorm"
)

func FindInfraApplyLikePattern(mysqlCli *gorm.DB, query map[string]interface{},
	search map[string]interface{}, limit, offset int32) ([]model.InfraApply, int, error) {
	S := make(map[string]interface{})
	for k, v := range search {
		field := k + " LIKE ?"
		S[field] = "%" + v.(string) + "%"
	}

	res, total, err := common.FindLike(mysqlCli, &model.InfraApply{}, query, S, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	return res.([]model.InfraApply), total, nil
}

//func CheckUserHasPermission(mysqlCli *gorm.DB, uid, serviceName string) (bool, error) {
//	ok, err := IsSuperAdmin(mysqlCli, uid)
//	if err != nil {
//		return false, err
//	}
//
//	// if is super admin, has all permission
//	if ok {
//		return true, nil
//	}
//
//	var a model.Admin
//	var db = mysqlCli.Table(a.TableName())
//	err = db.Where("uid=? AND service_name=?", uid, serviceName).Find(&a).Error
//	if err != nil {
//		if gorm.IsRecordNotFoundError(err) {
//			return false, nil
//		}
//		return false, err
//	}
//
//	return true, nil
//}

func FindOneInfraApply(mysqlCli *gorm.DB, query map[string]interface{}) (*model.InfraApply, error) {
	res, total, err := common.Find(mysqlCli, &model.InfraApply{}, query, 1, 0)
	if err != nil {
		return nil, err
	}

	if total <= 0 {
		return nil, nil
	}

	return &res.([]model.InfraApply)[0], nil
}

func UpdateInfraApply(mysqlCli *gorm.DB, ia *model.InfraApply, m map[string]interface{}) error {
	var db = mysqlCli.Model(ia)
	return db.Updates(m).Error
}
