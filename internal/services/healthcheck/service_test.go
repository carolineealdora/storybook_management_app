package healthcheck

import (
	"context"
	"testing"

	"codebase/config"
	"codebase/models"
	repository_mock "codebase/test/mock/repository"

	"github.com/labstack/echo/v4"
)

func TestNewViewService(t *testing.T) {
	type args struct {
		ctx          context.Context
		sharedHolder *models.SharedHolder
	}
	var (
		mockRepo        = new(repository_mock.MockHealthCheckRepository)
		ctx             = context.Background()
		viewServiceImpl = service{
			sharedHolder: &models.SharedHolder{
				Config: &config.SystemConfig{},
				Log:    nil,
				Echo:   &echo.Echo{},
				Sql:    nil,
			},
			repositoryHolder: repositoryHolder{
				HealthcheckRepository: mockRepo,
			},
			now: nil,
		}
	)
	tests := []struct {
		name    string
		args    args
		want    ViewService
		wantErr bool
	}{
		{
			name: "Test Case Positive - Return Success Healtcheck Repository Service",
			args: args{
				ctx:          ctx,
				sharedHolder: viewServiceImpl.sharedHolder,
			},
			want:    &viewServiceImpl,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewViewService(tt.args.ctx, tt.args.sharedHolder)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewViewService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
