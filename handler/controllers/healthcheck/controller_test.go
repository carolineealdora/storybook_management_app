package healthcheck

import (
	"testing"

	"codebase/models"
	helper_mocks "codebase/test/mock/helper"
	services_mocks "codebase/test/mock/service"
)

func TestNewController(t *testing.T) {
	type args struct {
		holder *models.SharedHolder
	}
	var (
		serviceMock    = new(services_mocks.ViewHealthCheckService)
		response       = new(helper_mocks.Response)
		controllerMock = Controller{
			HealthcheckService: serviceMock,
			Response:           response,
		}
	)
	tests := []struct {
		name string
		args args
		want Controller
	}{
		{
			name: "Test Case Positive - Success Return HealthCheck Controller",
			args: args{
				holder: &models.SharedHolder{},
			},
			want: controllerMock,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewController(tt.args.holder)
			if got.HealthcheckService == nil {
				t.Errorf("HealthcheckService is nil")
			}
		})

	}
}
