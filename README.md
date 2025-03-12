# Video Optimization Service

This is a simple video optimization service built with Go and Docker specifically for the CMS implementation of [Wedding Day Content Co.](https://weddingdaycontent.co). The service downloads videos from Cloudflare R2, optimizes them using [ffmpeg](https://www.ffmpeg.org/), generates thumbnails, and then uploads the processed files back to R2. It also provides endpoints for deleting files.

## Usage

### Endpoints

#### `/optimize`

##### Method `POST`

Accepts a JSON payload with the filename and options (such as desired resolution and format) to optimize a video.

Example `body`:

```json
{
  "filename": "example.mp4",
  "options": {
    "resolution": "720p",
    "format": "webm"
  }
}
```

#### `/delete`

##### Method `POST`

Accepts a JSON payload with an array of filenames to delete from S3.

Example `body`:

```json
{
  "filenames": ["example_optimized.webm", "example_thumbnail.png"]
}
```

#### `/health`

##### Method `GET`

A simple health check endpoint that returns `OK`.

### Authentication

Every request to `/optimize` and `/delete` must include an `Authorization` header.

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
