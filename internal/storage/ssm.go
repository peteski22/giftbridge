package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// SSMAPI defines the SSM operations used by the state store.
type SSMAPI interface {
	// GetParameter retrieves a parameter from SSM.
	GetParameter(
		ctx context.Context,
		params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.GetParameterOutput, error)

	// PutParameter stores a parameter in SSM.
	PutParameter(
		ctx context.Context,
		params *ssm.PutParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.PutParameterOutput, error)
}

// StateStore manages sync state in AWS SSM Parameter Store.
type StateStore struct {
	// client is the SSM API client.
	client SSMAPI

	// parameterName is the SSM parameter name.
	parameterName string
}

// LastSyncTime returns the timestamp of the last successful sync.
func (s *StateStore) LastSyncTime(ctx context.Context) (time.Time, error) {
	output, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(s.parameterName),
	})
	if err != nil {
		// Parameter not found is not an error - return zero time.
		var notFoundErr *types.ParameterNotFound
		if errors.As(err, &notFoundErr) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("getting parameter from SSM: %w", err)
	}

	if output.Parameter == nil || output.Parameter.Value == nil {
		return time.Time{}, nil
	}

	t, err := time.Parse(time.RFC3339, *output.Parameter.Value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing time from parameter: %w", err)
	}

	return t, nil
}

// SetLastSyncTime updates the last sync timestamp.
func (s *StateStore) SetLastSyncTime(ctx context.Context, t time.Time) error {
	_, err := s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.parameterName),
		Overwrite: aws.Bool(true),
		Type:      types.ParameterTypeString,
		Value:     aws.String(t.Format(time.RFC3339)),
	})
	if err != nil {
		return fmt.Errorf("putting parameter to SSM: %w", err)
	}

	return nil
}

// NewStateStore creates a new SSM-backed state store.
func NewStateStore(client SSMAPI, parameterName string) (*StateStore, error) {
	if client == nil {
		return nil, errors.New("ssm client is required")
	}
	if parameterName == "" {
		return nil, errors.New("parameter name is required")
	}

	return &StateStore{
		client:        client,
		parameterName: parameterName,
	}, nil
}
