package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/require"
)

type mockSSMClient struct {
	getParameterFunc func(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	putParameterFunc func(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

func (m *mockSSMClient) GetParameter(
	ctx context.Context,
	params *ssm.GetParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.GetParameterOutput, error) {
	if m.getParameterFunc != nil {
		return m.getParameterFunc(ctx, params, optFns...)
	}
	return &ssm.GetParameterOutput{}, nil
}

func (m *mockSSMClient) PutParameter(
	ctx context.Context,
	params *ssm.PutParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.PutParameterOutput, error) {
	if m.putParameterFunc != nil {
		return m.putParameterFunc(ctx, params, optFns...)
	}
	return &ssm.PutParameterOutput{}, nil
}

func TestNewStateStore(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		client        SSMAPI
		errMsg        string
		parameterName string
		wantErr       bool
	}{
		"valid inputs": {
			client:        &mockSSMClient{},
			parameterName: "/app/last-sync",
			wantErr:       false,
		},
		"nil client": {
			client:        nil,
			parameterName: "/app/last-sync",
			wantErr:       true,
			errMsg:        "ssm client is required",
		},
		"empty parameter name": {
			client:        &mockSSMClient{},
			parameterName: "",
			wantErr:       true,
			errMsg:        "parameter name is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, err := NewStateStore(tc.client, tc.parameterName)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
				require.Nil(t, store)
			} else {
				require.NoError(t, err)
				require.NotNil(t, store)
			}
		})
	}
}

func TestStateStore_LastSyncTime(t *testing.T) {
	t.Parallel()

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := map[string]struct {
		client  *mockSSMClient
		errMsg  string
		want    time.Time
		wantErr bool
	}{
		"returns time when found": {
			client: &mockSSMClient{
				getParameterFunc: func(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return &ssm.GetParameterOutput{
						Parameter: &types.Parameter{
							Value: aws.String("2024-01-15T10:30:00Z"),
						},
					}, nil
				},
			},
			want:    testTime,
			wantErr: false,
		},
		"returns zero time when parameter not found": {
			client: &mockSSMClient{
				getParameterFunc: func(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return nil, &types.ParameterNotFound{}
				},
			},
			want:    time.Time{},
			wantErr: false,
		},
		"returns zero time when parameter is nil": {
			client: &mockSSMClient{
				getParameterFunc: func(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return &ssm.GetParameterOutput{Parameter: nil}, nil
				},
			},
			want:    time.Time{},
			wantErr: false,
		},
		"returns zero time when value is nil": {
			client: &mockSSMClient{
				getParameterFunc: func(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return &ssm.GetParameterOutput{
						Parameter: &types.Parameter{Value: nil},
					}, nil
				},
			},
			want:    time.Time{},
			wantErr: false,
		},
		"returns error on invalid time format": {
			client: &mockSSMClient{
				getParameterFunc: func(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return &ssm.GetParameterOutput{
						Parameter: &types.Parameter{
							Value: aws.String("invalid-time"),
						},
					}, nil
				},
			},
			wantErr: true,
			errMsg:  "parsing time from parameter",
		},
		"returns error on ssm error": {
			client: &mockSSMClient{
				getParameterFunc: func(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
					return nil, errors.New("ssm error")
				},
			},
			wantErr: true,
			errMsg:  "getting parameter from SSM",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, err := NewStateStore(tc.client, "/app/last-sync")
			require.NoError(t, err)

			got, err := store.LastSyncTime(context.Background())

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestStateStore_SetLastSyncTime(t *testing.T) {
	t.Parallel()

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := map[string]struct {
		client  *mockSSMClient
		errMsg  string
		time    time.Time
		wantErr bool
	}{
		"successful set": {
			client: &mockSSMClient{
				putParameterFunc: func(_ context.Context, params *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
					require.Equal(t, "2024-01-15T10:30:00Z", *params.Value)
					require.True(t, *params.Overwrite)
					return &ssm.PutParameterOutput{}, nil
				},
			},
			time:    testTime,
			wantErr: false,
		},
		"ssm error": {
			client: &mockSSMClient{
				putParameterFunc: func(_ context.Context, _ *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
					return nil, errors.New("ssm error")
				},
			},
			time:    testTime,
			wantErr: true,
			errMsg:  "putting parameter to SSM",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, err := NewStateStore(tc.client, "/app/last-sync")
			require.NoError(t, err)

			err = store.SetLastSyncTime(context.Background(), tc.time)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
