package tgc

import (
	"context"
	"errors"
	"testing"

	"github.com/tgdrive/teldrive/internal/cache"
	jetmodel "github.com/tgdrive/teldrive/internal/database/jet/gen/model"
	"github.com/tgdrive/teldrive/pkg/repositories"
)

func TestAddBotsToNewChannelAllowsEmptyBotTokens(t *testing.T) {
	ctx := context.Background()
	cm := &ChannelManager{
		repo: &repositories.Repositories{
			Bots: fakeBotRepository{tokens: []string{}},
		},
		cache: cache.NewMemoryCache(1024 * 1024),
	}

	if err := cm.addBotsToNewChannel(ctx, 1234, 5678); err != nil {
		t.Fatalf("add bots to new channel with empty tokens: %v", err)
	}
}

func TestAddBotsToNewChannelPreservesBotTokensError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("load bot tokens")
	cm := &ChannelManager{
		repo: &repositories.Repositories{
			Bots: fakeBotRepository{err: expectedErr},
		},
		cache: cache.NewMemoryCache(1024 * 1024),
	}

	if err := cm.addBotsToNewChannel(ctx, 1234, 5678); !errors.Is(err, expectedErr) {
		t.Fatalf("expected BotTokens error %v, got %v", expectedErr, err)
	}
}

type fakeBotRepository struct {
	tokens []string
	err    error
}

func (r fakeBotRepository) CreateToken(context.Context, int64, string) error {
	return nil
}

func (r fakeBotRepository) Create(context.Context, *jetmodel.Bots) error {
	return nil
}

func (r fakeBotRepository) GetByUserID(context.Context, int64) ([]jetmodel.Bots, error) {
	return nil, nil
}

func (r fakeBotRepository) GetTokensByUserID(context.Context, int64) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.tokens, nil
}

func (r fakeBotRepository) DeleteByUserID(context.Context, int64) error {
	return nil
}
