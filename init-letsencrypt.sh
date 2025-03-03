#!/bin/bash

if [ "$STAGING" = "1" ]; then
    staging_arg="--staging"
else
    staging_arg=""
fi

export IFS=","
for domain in $DOMAINS; do
    if [ -d "/etc/letsencrypt/live/$domain" ]; then
        echo "Certificates already exists for $domain, renew checking..."
        certbot renew
    else
        echo "First install of certificates for $domain..."
        certbot certonly --webroot --webroot-path=/var/www/certbot \
            --email "$EMAIL" \
            --agree-tos \
            --no-eff-email \
            $staging_arg \
            -d $domain
    fi
done
