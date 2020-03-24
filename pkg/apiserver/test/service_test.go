package test

import (
	v1 "big-infra/pkg/apiserver/api/v1"
	logger "github.com/sirupsen/logrus"
	"testing"
)

func TestListInfraApply(t *testing.T) {
	req := v1.ListInfraApplyReq{
		PageIdx:  1,
		PageSize: -1,
	}
	_, err := InfraCli.cli.ListInfraApply(InfraCli.ctx, &req)
	if err != nil {
		t.Error(err.Error())
	}
	//logger.Infof("%+v", resp)
}

func TestUpdateInfraApply(t *testing.T) {
	req := v1.UpdateInfraApplyReq{
		ID:     13,
		Status: "审批完成",
	}
	resp, err := InfraCli.cli.UpdateInfraApply(InfraCli.ctx, &req)
	if err != nil {
		t.Error(err.Error())
	}
	logger.Infof("%+v", resp)
}
