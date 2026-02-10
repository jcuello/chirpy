-- +goose Up
CREATE TABLE chirps(
  id UUID PRIMARY KEY,
  created_at timestamp,
  updated_at timestamp,
  body text,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE chirps;