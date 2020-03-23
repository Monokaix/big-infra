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
