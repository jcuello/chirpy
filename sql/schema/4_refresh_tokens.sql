-- +goose Up
CREATE TABLE refresh_tokens(
  token TEXT PRIMARY KEY,
  created_at timestamp,
  updated_at timestamp,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  expires_at timestamp,
  revoked_at timestamp
);

-- +goose Down
DROP TABLE refresh_tokens;