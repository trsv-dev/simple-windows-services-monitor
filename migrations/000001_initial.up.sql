CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    login VARCHAR(250) NOT NULL UNIQUE,
    password VARCHAR(250) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS servers (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    name VARCHAR(250) NOT NULL,
    address VARCHAR(250) NOT NULL,
    username VARCHAR(250) NOT NULL,
    password TEXT NOT NULL,
    fingerprint UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT unique_user_server UNIQUE (user_id, address)
);

CREATE INDEX idx_servers_user_id ON servers(user_id);
CREATE INDEX idx_servers_fingerprint ON servers(fingerprint);

CREATE TABLE IF NOT EXISTS services (
    id BIGSERIAL PRIMARY KEY,
    server_id INTEGER NOT NULL,
    displayed_name VARCHAR(250) NOT NULL,
    service_name VARCHAR(250) NOT NULL,
    status VARCHAR(250) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
    CONSTRAINT unique_service_server UNIQUE (server_id, service_name)
);
