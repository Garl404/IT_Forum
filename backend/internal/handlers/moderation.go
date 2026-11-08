package handlers

import (
	"github.com/Garl404/IT_Forum/backend/internal/moderation"
	"github.com/Garl404/IT_Forum/backend/internal/models"
)

func moderation_List() []models.BannedWord { return moderation.Default.List() }
func moderation_Count() int                { return len(moderation.Default.List()) }
func moderation_Reload(path string) error  { return moderation.Default.Reload(path) }
