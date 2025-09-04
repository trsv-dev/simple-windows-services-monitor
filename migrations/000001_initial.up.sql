CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    login VARCHAR(250) NOT NULL UNIQUE,
    password VARCHAR(250) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS servers (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    name VARCHAR(250) NOT NULL,
    address VARCHAR(250) NOT NULL,
    username VARCHAR(250) NOT NULL,   -- учётная запись для WinRM
    password TEXT NOT NULL, -- зашифрованный пароль для WinRM
--  last_check TIMESTAMPTZ,           -- когда последний раз проверяли
--  online BOOLEAN DEFAULT false,     -- текущий статус соединения
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT unique_user_server UNIQUE (user_id, address)
);

CREATE TABLE IF NOT EXISTS services (
    id SERIAL PRIMARY KEY,
    server_id INTEGER NOT NULL,
    displayed_name VARCHAR(250) NOT NULL,
    service_name VARCHAR(250) NOT NULL,
    status VARCHAR(250) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
);
