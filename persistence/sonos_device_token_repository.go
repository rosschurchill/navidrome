package persistence

import (
	"context"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type sonosDeviceTokenRepository struct {
	sqlRepository
}

func NewSonosDeviceTokenRepository(ctx context.Context, db dbx.Builder) model.SonosDeviceTokenRepository {
	r := &sonosDeviceTokenRepository{}
	r.ctx = ctx
	r.db = db
	r.registerModel(&model.SonosDeviceToken{}, nil)
	return r
}

func (r *sonosDeviceTokenRepository) Get(id string) (*model.SonosDeviceToken, error) {
	sel := r.newSelect().Where(Eq{"id": id})
	var res model.SonosDeviceToken
	err := r.queryOne(sel, &res)
	return &res, err
}

func (r *sonosDeviceTokenRepository) GetByToken(token string) (*model.SonosDeviceToken, error) {
	sel := r.newSelect().Where(Eq{"token": token})
	var res model.SonosDeviceToken
	err := r.queryOne(sel, &res)
	return &res, err
}

func (r *sonosDeviceTokenRepository) GetByUserID(userID string) (model.SonosDeviceTokens, error) {
	sel := r.newSelect().Where(Eq{"user_id": userID}).OrderBy("created_at desc")
	var res model.SonosDeviceTokens
	err := r.queryAll(sel, &res)
	return res, err
}

func (r *sonosDeviceTokenRepository) GetByHouseholdID(householdID string) (model.SonosDeviceTokens, error) {
	sel := r.newSelect().Where(Eq{"household_id": householdID}).OrderBy("created_at desc")
	var res model.SonosDeviceTokens
	err := r.queryAll(sel, &res)
	return res, err
}

func (r *sonosDeviceTokenRepository) Put(token *model.SonosDeviceToken) error {
	token.UpdatedAt = time.Now()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = token.UpdatedAt
	}
	_, err := r.put(token.ID, token)
	return err
}

func (r *sonosDeviceTokenRepository) Delete(id string) error {
	return r.delete(Eq{"id": id})
}

func (r *sonosDeviceTokenRepository) DeleteByUserID(userID string) error {
	return r.delete(Eq{"user_id": userID})
}

func (r *sonosDeviceTokenRepository) UpdateLastSeen(id string, lastSeen time.Time) error {
	upd := Update(r.tableName).
		Set("last_seen_at", lastSeen).
		Set("updated_at", time.Now()).
		Where(Eq{"id": id})
	_, err := r.executeSQL(upd)
	return err
}

func (r *sonosDeviceTokenRepository) CountAll(options ...model.QueryOptions) (int64, error) {
	return r.count(r.newSelect(), options...)
}

var _ model.SonosDeviceTokenRepository = (*sonosDeviceTokenRepository)(nil)
