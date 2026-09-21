package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"
	"uuid"

	"github.com/labstack/echo/v5"

	"github.com/sudeeya/gophprofile/internal/domain"
	"github.com/sudeeya/gophprofile/internal/services"
)

const UserIDHeaderKey = "X-User-ID"

type AvatarService interface {
	UploadAvatar(ctx context.Context, input services.UploadAvatarInput) (domain.Avatar, error)
	GetAvatar(ctx context.Context, id uuid.UUID) (domain.Avatar, error)
	GetAvatarMetadata(ctx context.Context, id uuid.UUID) (domain.Metadata, error)
	DeleteAvatar(ctx context.Context, input services.DeleteAvatarInput) error
}

type AvatarHandler struct {
	service AvatarService
}

func NewAvatarHandler(service AvatarService) *AvatarHandler {
	return &AvatarHandler{
		service: service,
	}
}

type UploadAvatarResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type UploadAvatarError struct {
	Error   string `json:"error"`
	Details string `json:"details"`
	MaxSize int    `json:"max_size"`
}

func (h *AvatarHandler) UploadAvatar(c *echo.Context) error {
	userID := c.Request().Header.Get(UserIDHeaderKey)
	if userID == "" {
		return c.JSON(http.StatusBadRequest, UploadAvatarError{
			Error: "Missing X-User-ID header",
		})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, UploadAvatarError{
			Error: "Missing file",
		})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, UploadAvatarError{
			Error: "Internal server error",
		})
	}
	defer file.Close()

	avatar, err := h.service.UploadAvatar(c.Request().Context(), services.UploadAvatarInput{
		UserID:   userID,
		Filename: fileHeader.Filename,
		Reader:   file,
	})
	switch {
	case errors.Is(err, services.ErrAvatarTooLarge):
		return c.JSON(http.StatusRequestEntityTooLarge, UploadAvatarError{
			Error:   "File too large",
			MaxSize: services.MaxAvatarSize,
		})
	case errors.Is(err, services.ErrFormatNotSupported):
		return c.JSON(http.StatusBadRequest, UploadAvatarError{
			Error:   "Invalid file format",
			Details: "Supported formats: jpeg, png, webp",
		})
	case err != nil:
		return c.JSON(http.StatusInternalServerError, UploadAvatarError{
			Error: "Internal server error",
		})
	}

	return c.JSON(http.StatusCreated, UploadAvatarResponse{
		ID:        avatar.Metadata.ID,
		UserID:    avatar.Metadata.UserID,
		CreatedAt: avatar.Metadata.CreatedAt,
	})
}

type GetAvatarError struct {
	Error string `json:"error"`
}

func (h *AvatarHandler) GetAvatar(c *echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetAvatarError{
			Error: "Invalid avatar id",
		})
	}

	avatar, err := h.service.GetAvatar(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, GetAvatarError{
			Error: "Not found",
		})
	}

	return c.Blob(http.StatusOK, avatar.Metadata.MimeType, avatar.Bytes)
}

type GetAvatarMetadataError struct {
	Error string `json:"error"`
}

func (h *AvatarHandler) GetAvatarMetadata(c *echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, GetAvatarMetadataError{
			Error: "Invalid avatar id",
		})
	}

	metadata, err := h.service.GetAvatarMetadata(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, GetAvatarMetadataError{
			Error: "Not found",
		})
	}

	return c.JSON(http.StatusOK, metadata)
}

type DeleteAvatarError struct {
	Error   string `json:"error"`
	Details string `json:"details"`
}

func (h *AvatarHandler) DeleteAvatar(c *echo.Context) error {
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		return c.JSON(http.StatusBadRequest, DeleteAvatarError{
			Error: "Missing X-User-ID header",
		})
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, DeleteAvatarError{
			Error: "Invalid avatar id",
		})
	}

	err = h.service.DeleteAvatar(c.Request().Context(), services.DeleteAvatarInput{
		ID:     id,
		UserID: userID,
	})
	switch {
	case errors.Is(err, services.ErrForbidden):
		return c.JSON(http.StatusForbidden, DeleteAvatarError{
			Error:   "Forbidden",
			Details: "You can only delete your own avatars",
		})
	case err != nil:
		return c.JSON(http.StatusInternalServerError, DeleteAvatarError{
			Error: "Internal server error",
		})
	}

	return c.NoContent(http.StatusNoContent)
}
