package web

import (
	"net/http"

	"github.com/demostanis/42evaluators/internal/database"
	"github.com/demostanis/42evaluators/internal/models"
	"github.com/demostanis/42evaluators/web/templates"
	"gorm.io/gorm"
)

func handlePeerFinder(db *gorm.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var subjects []models.Subject

		err := db.
			Model(&models.Subject{}).
			Find(&subjects).Error
		if err != nil {
			internalServerError(w, err)
			return
		}

		i := 0
		var totalProjects []models.Project
		for {
			campusId := 62
			var projects []models.Project
			db.
				Where("status != 'finished'").
				Preload("Teams.Users.User",
					"campus_id = ? AND "+database.OnlyRealUsersCondition,
					campusId).
				Preload("Subject").
				Limit(10000).
				Offset(10000 * i).
				Model(&models.Project{}).
				Find(&projects)
			if len(projects) == 0 {
				break
			}
			totalProjects = append(totalProjects, projects...)
			i++
		}

		projectsMap := make(map[int][]models.Project)
		for _, project := range totalProjects {
			// filter by projects for which the preload condition succeeded
			if len(project.Teams) > 0 && len(project.Teams[0].Users) > 0 &&
				project.Teams[0].Users[0].User.ID != 0 {
				projectsMap[project.Subject.ID] = append(
					projectsMap[project.Subject.ID], project)
			}
		}

		templates.PeerFinder(subjects, projectsMap).
			Render(r.Context(), w)
	})
}
