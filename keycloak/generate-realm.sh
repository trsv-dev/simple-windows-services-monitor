#!/bin/bash
set -e

echo "Generating realm..."

mkdir -p /opt/keycloak/data/import

# создаем realm.json из шаблона
cat /opt/keycloak/realm-template.json > /opt/keycloak/data/import/realm.json

# подставляем переменные
for var in KEYCLOAK_REALM_NAME KEYCLOAK_CLIENT_ID ADMIN_USERNAME ADMIN_PASSWORD ADMIN_EMAIL SMTP_FROM SMTP_FROM_NAME SMTP_HOST SMTP_PORT SMTP_USERNAME SMTP_PASSWORD; do
    sed -i "s|\${$var}|${!var}|g" /opt/keycloak/data/import/realm.json
done

echo "Realm generated"

# экспорт переменных для bootstrap admin
export KC_BOOTSTRAP_ADMIN_USERNAME=$ADMIN_USERNAME
export KC_BOOTSTRAP_ADMIN_PASSWORD=$ADMIN_PASSWORD
export KC_BOOTSTRAP_ADMIN_EMAIL=$ADMIN_EMAIL

echo "Bootstrap admin will be created automatically on Keycloak start"