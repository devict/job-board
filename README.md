# devICT job board

a go rewrite of the original [devICT job board](https://github.com/devict/jobs.devict) by [@imacrayon](https://github.com/imacrayon), design and original feature set came from there!

## requirements

- docker + docker compose
- that's mainly it!
- for a nicer local dev experience, having go and some go IDE integration set up is likely worthwhile

## setup for development

- Copy `.env.example` to `.env` and make any desired changes
- (If on Windows) Make sure your local repository path (for example `C:\Users\username\repos\job-board`) is available for Docker Filesharing by going to `Docker Dashboard > Settings > Resources > File Sharing`

## running locally

1. Run docker

    On Linux
    ```shell
    $ docker compose up
    ```

    On Windows:
    ```shell
    $ docker-compose up
    ```
1. Open `localhost:8080` in the browser


## accessing the database

```shell
$ make psql
```

## seed the database with test data

```shell
$ make seed-db
```

## slack integration

setting the `SLACK_HOOK` env var will enable posting new jobs to Slack to the provided Slack hook url. if not configured, this functionality will simply be disabled

## email integration

for testing email sending locally, it is recommended that you use [mailtrap](http://mailtrap.io), then copy `.env.example` to `.env` and add your configuration there

## database migrations

[golang-migrate](https://github.com/golang-migrate/migrate) is used for db migrations. the server runs migrations as it starts up, so unless you're adding new migrations or doing other stuff with the migration files you shouldn't have to worry about this tool.
