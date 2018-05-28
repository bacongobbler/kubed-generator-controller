FROM node:8

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

ARG NODE_ENV
ENV NODE_ENV $NODE_ENV
COPY package.json /usr/src/app/
RUN yarn install && yarn cache clean --force
COPY . /usr/src/app

ENV PORT 8080
EXPOSE 8080

CMD [ "yarn", "start" ]
