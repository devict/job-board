CREATE TABLE IF NOT EXISTS roles (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT NOT NULL,
  phone TEXT NULL,
  role TEXT NOT NULL,
  resume TEXT NOT NULL,
  linkedin TEXT NULL,
  website TEXT NULL,
  github TEXT NULL,
  comp_low TEXT NULL,
  comp_high TEXT NULL,
  published_at TIMESTAMP DEFAULT current_timestamp
);