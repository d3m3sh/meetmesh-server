#!/bin/bash

if [ "$STAGING" = "1" ]; then
    staging_arg="--staging"
else
    staging_arg=""
fi

domain=$(echo "$DOMAINS" | cut -d',' -f1)
if [ -d "/etc/letsencrypt/live/$domain" ]; then
    echo "Certificates already exists, renew checking..."
    certbot renew
else
    echo "First install of certificates..."
    certbot certonly --webroot --webroot-path=/var/www/certbot \
        --email "$EMAIL" \
        --agree-tos \
        --no-eff-email \
        $staging_arg \
        -d $domain
fi 