FROM oryd/keto:latest
FROM alpine:3.9
COPY --from=0 /usr/bin/keto /usr/bin/


ENV DSN=memory
ADD clinic_policy.json .
ENTRYPOINT []
CMD exec /bin/sh -c "trap : TERM INT; keto serve &  sleep 2; keto --endpoint=http://localhost:4466  engines acp ory policies import glob clinic_policy.json; (while true; do sleep 1000; done) & wait"
