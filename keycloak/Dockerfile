FROM quay.io/phasetwo/phasetwo-keycloak:26.5.6

USER root

COPY realm-template.testjson /opt/keycloak/realm-template.json
COPY generate-realm.sh /opt/keycloak/generate-realm.sh

RUN chmod +x /opt/keycloak/generate-realm.sh

USER 1000

ENTRYPOINT ["/bin/bash", "-c", "/opt/keycloak/generate-realm.sh && /opt/keycloak/bin/kc.sh start-dev --import-realm --db=dev-file --spi-email-template-provider=freemarker-plus-mustache --spi-email-template-freemarker-plus-mustache-enabled=true --spi-theme-cache-themes=false"]