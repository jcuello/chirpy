-- +goose Up
CREATE TABLE users(
  id UUID PRIMARY KEY,
  created_at timestamp,
  updated_at timestamp,
  email text
);

-- +goose Down
DROP TABLE users;