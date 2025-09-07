FROM node:24-alpine AS builder
WORKDIR /app
COPY ./nivek-vue .
RUN npm install --no-fund
CMD ["npm", "run", "dev", "--", "--host"]
