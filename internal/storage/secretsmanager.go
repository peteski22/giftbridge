package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// SecretsManagerAPI defines the Secrets Manager operations used by the token store.
type SecretsManagerAPI interface {
	// GetSecretValue retrieves a secret value.
	GetSecretValue(
		ctx context.Context,
		params *secretsmanager.GetSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.GetSecretValueOutput, error)

	// PutSecretValue stores a secret value.
	PutSecretValue(
		ctx context.Context,
		params *secretsmanager.PutSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.PutSecretValueOutput, error)
}

// TokenStore manages OAuth refresh tokens in AWS Secrets Manager.
type TokenStore struct {
	// client is the Secrets Manager API client.
	client SecretsManagerAPI

	// secretARN is the ARN of the secret storing the refresh token.
	secretARN string
}

// RefreshToken returns the current refresh token from Secrets Manager.
func (t *TokenStore) RefreshToken(ctx context.Context) (string, error) {
	output, err := t.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(t.secretARN),
	})
	if err != nil {
		return "", fmt.Errorf("getting secret from Secrets Manager: %w", err)
	}

	if output.SecretString == nil {
		return "", errors.New("secret has no string value")
	}

	return *output.SecretString, nil
}

// SaveRefreshToken stores a new refresh token in Secrets Manager.
func (t *TokenStore) SaveRefreshToken(ctx context.Context, token string) error {
	if token == "" {
		return errors.New("token cannot be empty")
	}

	_, err := t.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(t.secretARN),
		SecretString: aws.String(token),
	})
	if err != nil {
		return fmt.Errorf("putting secret to Secrets Manager: %w", err)
	}

	return nil
}

// NewTokenStore creates a new Secrets Manager-backed token store.
func NewTokenStore(client SecretsManagerAPI, secretARN string) (*TokenStore, error) {
	if client == nil {
		return nil, errors.New("secrets manager client is required")
	}
	if secretARN == "" {
		return nil, errors.New("secret ARN is required")
	}

	return &TokenStore{
		client:    client,
		secretARN: secretARN,
	}, nil
}
