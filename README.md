# Video Service

This is a simple video service built with Go and Docker specifically for the CMS implementation of [Wedding Day Content Co.](https://weddingdaycontent.co). The service downloads videos from Cloudflare R2, generates thumbnails using [ffmpeg](https://www.ffmpeg.org/), and then uploads the thumbnails back to R2, while also providing an endpoint to delete files.

## Usage

### Endpoints

#### `/thumbnail`

##### Method `POST`

Accepts a JSON payload with the filename to generate a thumbnail for.

Example `body`:

```json
{
  "filename": "example.mp4"
}
```

#### `/delete`

##### Method `POST`

Accepts a JSON payload with an array of filenames to delete from S3.

Example `body`:

```json
{
  "filenames": ["example_thumbnail.png"]
}
```

#### `/health`

##### Method `GET`

A simple health check endpoint that returns `OK`.

### Authentication

Every request to `/thumbnail` and `/delete` must include an `Authorization` header.

Example `headers`:

```json
{
  "Authorization": "API-KEY <api-key>"
}
```

The API key is verified using a constant-time string comparison to mitigate timing attacks.

## Deployment

### Build Docker Image

```zsh
docker build -t video-optimization .
```

### Run Docker Image with Environment Variables

- `R2_ENDPOINT` Cloudflare R2 endpoint
- `R2_BUCKET` Cloudflare R2 bucket name
- `R2_ACCESS_KEY_ID` Cloudflare R2 access key ID
- `R2_SECRET_ACCESS_KEY` Cloudflare R2 secret access key
- `SERVER_URL` URL of the CMS
- `VIDEO_OPTIMIZATION_API_KEY` API key for authentication
- `APP_ENV` (optional, set to `development` or `production` to adjust logging behavior)

```zsh
docker run -p 8080:8080 \
  -e R2_ENDPOINT= \
  -e R2_BUCKET= \
  -e R2_ACCESS_KEY_ID= \
  -e R2_SECRET_ACCESS_KEY= \
  -e SERVER_URL= \
  -e VIDEO_OPTIMIZATION_API_KEY= \
  video-optimization
```
