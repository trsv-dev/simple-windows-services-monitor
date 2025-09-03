DROP TABLE services; --Так как у нас services ссылается на servers, то сначала удаляем services
DROP TABLE servers; --Так как у нас servers ссылается на users, то сначала удаляем servers
DROP TABLE users;