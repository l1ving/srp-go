package main

import (
	"bytes"
	"github.com/valyala/fasthttp"
	"strconv"
	"time"
)

var (
	uploadPath = []byte("/api/upload")
)

func HandleApi(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		HandleGeneric(ctx, fasthttp.StatusMethodNotAllowed, "Cannot "+string(ctx.Method())+" on /api/")
		return
	}

	if *allowUpload != true {
		HandleForbidden(ctx)
		return
	}

	path := ctx.Path()

	switch {
	case bytes.Equal(path, uploadPath):
		handleUpload(ctx)
	}
}

func handleUpload(ctx *fasthttp.RequestCtx) {
	fh, err := ctx.FormFile("file")
	tmpName := strconv.FormatInt(time.Now().UnixNano(), 10)
	path := "www/content/tmp/" + tmpName

	if err == nil {
		err = fasthttp.SaveMultipartFile(fh, path)
		if err != nil {
			HandleInternalServerError(ctx, "Error saving file from /api/upload", err)
			return
		}

		image, err := SaveFinal(path)
		if err != nil {
			HandleInternalServerError(ctx, "Error converting file from /api/upload", err)
			return
		}

		ctx.Response.Header.Set("X-Image-Hash", image)
		HandleGeneric(ctx, fasthttp.StatusCreated, "Created")

		// Update image cache after uploading a new image
		// we want to check if it's missing in case the user uploads the same image more than once
		imageCache = AppendIfMissing(imageCache, image)
		// Update the browse gallery cache after uploading
		galleryCache = LoadGalleryCache()
	} else {
		HandleInternalServerError(ctx, "Other error with handling upload", err)
	}
}
