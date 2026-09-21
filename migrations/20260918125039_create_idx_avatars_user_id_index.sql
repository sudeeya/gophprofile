-- +goose Up
CREATE INDEX idx_avatars_user_id ON avatars(user_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX idx_avatars_user_id;
