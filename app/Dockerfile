FROM node:23-alpine AS build-stage

ARG ENVIRONMENT=local

WORKDIR /app

# Install dependencies separately for better caching
COPY package.json package-lock.json* ./
RUN \
    if [ -f package-lock.json ]; then npm ci; \
    else npm install; fi

COPY . .

COPY env/.env.$ENVIRONMENT .env

RUN npm run build

FROM nginx:1.29.5-alpine

# Pull security patches for OS packages
RUN apk update --no-cache && apk upgrade --no-cache

# Remove default nginx static assets
RUN rm -rf /usr/share/nginx/html/*

# Copy built assets from build-stage
COPY --from=build-stage /app/dist /usr/share/nginx/html/app

COPY nginx.conf /etc/nginx/nginx.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
