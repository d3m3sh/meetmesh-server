# BUILD
`go build -o signaling .`

# BUILD DOCKER IMG
`bash build.sh [tag]`

# DOCKER-COMPOSE
## add .env file

```
DOMAIN_NAME=XXXX
DOMAINS=XXX, YYYY
EMAIL=XXX@XXX
STAGING=0  # 1 to staging, 0 to production 
```

## UP
`docker-compose up -d`