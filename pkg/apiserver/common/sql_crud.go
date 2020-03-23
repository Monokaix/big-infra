package common

import (
	"errors"
	"reflect"

	"big-infra/pkg/model"

	"github.com/jinzhu/gorm"
)

func getTableName(record interface{}) (string, error) {
	var tableName string
	switch aRecord := record.(type) {
	default:
		return "", errors.New("Unrecognized db model")
	case *model.InfraApply:
		tableName = aRecord.TableName()
	}
	return tableName, nil
}

func AddOne(MysqlCli *gorm.DB, aRecord interface{}) error {
	tableName, err := getTableName(aRecord)
	if err != nil {
		return err
	}

	var db = MysqlCli.Table(tableName)

	return db.Create(aRecord).Error
}

// DeleteOne deletes a single record matching the keys provided. Returns error if multiple records found.
func DeleteOne(MysqlCli *gorm.DB, aRecord interface{}) error {
	tableName, err := getTableName(aRecord)
	if err != nil {
		return err
	}

	var cnt int
	record := reflect.ValueOf(aRecord).Interface()
	var db = MysqlCli.Table(tableName)
	db.Where(aRecord).Find(record).Count(&cnt)

	if cnt > 1 {
		return errors.New("multiple records found for the keywords provided")
	}
	return db.Delete(record).Error
}

func UpdateOne(MysqlCli *gorm.DB, aRecord interface{}, changedFields map[string]interface{}) error {
	tableName, err := getTableName(aRecord)
	if err != nil {
		return err
	}

	db := MysqlCli
	record := reflect.ValueOf(aRecord).Interface()
	var cnt int
	db.Table(tableName).Where(aRecord).Find(record).Count(&cnt)

	if cnt > 1 {
		return errors.New("multiple records found for the keywords provided")
	}
	if cnt == 0 {
		return errors.New("no record found for the keywords provided")
	}

	return db.Model(record).Updates(changedFields).Error
}

func Find(MysqlCli *gorm.DB, dummyRecord interface{}, query map[string]interface{},
	limit, offset int32) (interface{}, int, error) {
	if limit < -1 || limit == 0 || limit > PAGE_SIZE {
		return nil, 0, errors.New("invalid page size")
	}
	if offset < -1 {
		return nil, 0, errors.New("offset cannot be negative")
	}

	tableName, err := getTableName(dummyRecord)
	if err != nil {
		return nil, 0, err
	}

	var db = MysqlCli.Table(tableName)
	modelType := reflect.Indirect(reflect.ValueOf(dummyRecord)).Type()
	records := reflect.New(reflect.SliceOf(modelType))
	err = db.Where(query).Limit(limit).Offset(offset).Find(records.Interface()).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			err = nil
		} else {
			return nil, 0, err
		}

	}
	var total int
	err = db.Where(query).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	return reflect.Indirect(records).Interface(), total, nil
}

// FindLike support `LIKE` in query with many fields, and the relation is `AND`
func FindLike(MysqlCli *gorm.DB, dummyRecord interface{}, query map[string]interface{},
	search map[string]interface{}, limit, offset int32) (interface{}, int, error) {
	if limit < -1 || limit == 0 || limit > PAGE_SIZE {
		return nil, 0, errors.New("invalid page size")
	}
	if offset < -1 {
		return nil, 0, errors.New("offset cannot be negative")
	}

	tableName, err := getTableName(dummyRecord)
	if err != nil {
		return nil, 0, err
	}

	var db = MysqlCli.Table(tableName)
	modelType := reflect.Indirect(reflect.ValueOf(dummyRecord)).Type()
	records := reflect.New(reflect.SliceOf(modelType))

	Q := db.Where(query)
	for k, v := range search {
		Q = Q.Where(k, v)
	}
	err = Q.Limit(limit).Offset(offset).Find(records.Interface()).Error
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = Q.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	return reflect.Indirect(records).Interface(), total, nil
}
