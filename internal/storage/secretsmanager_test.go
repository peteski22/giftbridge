package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/require"
)

type mockSecretsManagerAPI struct {
	getSecretValueFunc func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	putSecretValueFunc func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
}

func (m *mockSecretsManagerAPI) GetSecretValue(
	ctx context.Context,
	params *secretsmanager.GetSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	return m.getSecretValueFunc(ctx, params, optFns...)
}

func (m *mockSecretsManagerAPI) PutSecretValue(
	ctx context.Context,
	params *secretsmanager.PutSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.PutSecretValueOutput, error) {
	return m.putSecretValueFunc(ctx, params, optFns...)
}

func TestNewTokenStore(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		client    SecretsManagerAPI
		secretARN string
		wantErr   bool
		errMsg    string
	}{
		"valid inputs": {
			client:    &mockSecretsManagerAPI{},
			secretARN: "arn:aws:secretsmanager:us-east-1:123456789012:secret:test",
			wantErr:   false,
		},
		"nil client": {
			client:    nil,
			secretARN: "arn:aws:secretsmanager:us-east-1:123456789012:secret:test",
			wantErr:   true,
			errMsg:    "secrets manager client is required",
		},
		"empty secret ARN": {
			client:    &mockSecretsManagerAPI{},
			secretARN: "",
			wantErr:   true,
			errMsg:    "secret ARN is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, err := NewTokenStore(tc.client, tc.secretARN)

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

func TestTokenStore_RefreshToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupMock func() *mockSecretsManagerAPI
		wantToken string
		wantErr   bool
		errMsg    string
	}{
		"returns token successfully": {
			setupMock: func() *mockSecretsManagerAPI {
				return &mockSecretsManagerAPI{
					getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
						return &secretsmanager.GetSecretValueOutput{
							SecretString: aws.String("refresh-token-value"),
						}, nil
					},
				}
			},
			wantToken: "refresh-token-value",
			wantErr:   false,
		},
		"API error": {
			setupMock: func() *mockSecretsManagerAPI {
				return &mockSecretsManagerAPI{
					getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
						return nil, errors.New("access denied")
					},
				}
			},
			wantErr: true,
			errMsg:  "getting secret from Secrets Manager",
		},
		"nil secret string": {
			setupMock: func() *mockSecretsManagerAPI {
				return &mockSecretsManagerAPI{
					getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
						return &secretsmanager.GetSecretValueOutput{
							SecretString: nil,
						}, nil
					},
				}
			},
			wantErr: true,
			errMsg:  "secret has no string value",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mock := tc.setupMock()
			store, err := NewTokenStore(mock, "arn:aws:secretsmanager:us-east-1:123456789012:secret:test")
			require.NoError(t, err)

			token, err := store.RefreshToken(context.Background())

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantToken, token)
			}
		})
	}
}

func TestTokenStore_SaveRefreshToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setupMock func() *mockSecretsManagerAPI
		token     string
		wantErr   bool
		errMsg    string
	}{
		"saves token successfully": {
			setupMock: func() *mockSecretsManagerAPI {
				return &mockSecretsManagerAPI{
					putSecretValueFunc: func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
						return &secretsmanager.PutSecretValueOutput{}, nil
					},
				}
			},
			token:   "new-refresh-token",
			wantErr: false,
		},
		"empty token": {
			setupMock: func() *mockSecretsManagerAPI {
				return &mockSecretsManagerAPI{}
			},
			token:   "",
			wantErr: true,
			errMsg:  "token cannot be empty",
		},
		"API error": {
			setupMock: func() *mockSecretsManagerAPI {
				return &mockSecretsManagerAPI{
					putSecretValueFunc: func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
						return nil, errors.New("access denied")
					},
				}
			},
			token:   "new-refresh-token",
			wantErr: true,
			errMsg:  "putting secret to Secrets Manager",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mock := tc.setupMock()
			store, err := NewTokenStore(mock, "arn:aws:secretsmanager:us-east-1:123456789012:secret:test")
			require.NoError(t, err)

			err = store.SaveRefreshToken(context.Background(), tc.token)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
