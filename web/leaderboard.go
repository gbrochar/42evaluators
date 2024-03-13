package web

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/demostanis/42evaluators/internal/database"
	"github.com/demostanis/42evaluators/internal/models"
	"github.com/demostanis/42evaluators/web/templates"
	"gorm.io/gorm"
)

// Processes the user's ?fields= URL param by splitting it
// on commas and returning a map of valid (according to
// templates.ToggleableFields) templates.Fields.
func getShownFields(wantedFieldsRaw string) map[string]templates.Field {
	shownFields := make(map[string]templates.Field)

	wantedFields := []string{"level", "campus"}
	if wantedFieldsRaw != "" {
		wantedFields = strings.Split(wantedFieldsRaw, ",")
	}

	for _, field := range templates.ToggleableFields {
		found := false
		for _, allowedField := range wantedFields {
			if field.Name == allowedField {
				found = true
			}
		}
		shownFields[field.Name] = templates.Field{
			Name:       field.Name,
			PrettyName: field.PrettyName,
			Checked:    found,
		}
	}

	return shownFields
}

func canSortOn(field string) bool {
	_ = field
	return true
}

func getPromosForCampus(
	db *gorm.DB,
	campus string,
	promo string,
) ([]templates.Promo, error) {
	var promos []templates.Promo

	var campusUsers []models.User
	err := db.
		Scopes(database.WithCampus(campus)).
		Scopes(database.OnlyRealUsers()).
		Find(&campusUsers).Error
	if err != nil {
		return promos, err
	}

	for _, user := range campusUsers {
		userPromo := fmt.Sprintf("%02d/%d",
			user.BeginAt.Month(),
			user.BeginAt.Year())

		shouldAdd := true
		for _, alreadyAddedPromo := range promos {
			if userPromo == alreadyAddedPromo.Name {
				shouldAdd = false
				break
			}
		}
		if shouldAdd {
			promos = append(promos, templates.Promo{
				Name:   userPromo,
				Active: promo == userPromo,
			})
		}
	}

	slices.SortFunc(promos, func(a, b templates.Promo) int {
		parseDate := func(promo templates.Promo) (int, int) {
			parts := strings.Split(promo.Name, "/")
			month, _ := strconv.Atoi(parts[0])
			year, _ := strconv.Atoi(parts[1])
			return month, year
		}
		monthA, yearA := parseDate(a)
		monthB, yearB := parseDate(b)

		return (monthA | yearA<<5) - (monthB | yearB<<5)
	})

	return promos, nil
}

func getAllCampuses(db *gorm.DB) ([]models.Campus, error) {
	var campuses []models.Campus
	err := db.Find(&campuses).Error
	return campuses, err
}

func internalServerError(w http.ResponseWriter, err error) {
	_ = err
	w.WriteHeader(http.StatusInternalServerError)
}

func handleLeaderboard(db *gorm.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page <= 0 {
			page = 1
		}

		sorting := r.URL.Query().Get("sort")
		if sorting == "" || !canSortOn(sorting) {
			sorting = "level"
		}

		promo := r.URL.Query().Get("promo")
		campus := r.URL.Query().Get("campus")
		shownFields := getShownFields(r.URL.Query().Get("fields"))

		campuses, err := getAllCampuses(db)
		if err != nil {
			internalServerError(w, err)
			return
		}

		promos, err := getPromosForCampus(db, campus, promo)
		if err != nil {
			internalServerError(w, err)
			return
		}

		var users []models.User

		var totalUsers int64
		err = db.
			Model(&models.User{}).
			Scopes(database.OnlyRealUsers()).
			Scopes(database.WithCampus(campus)).
			Scopes(database.WithPromo(promo)).
			Count(&totalUsers).Error
		if err != nil {
			internalServerError(w, err)
			return
		}
		totalPages := 1 + (int(totalUsers)-1)/UsersPerPage
		page = min(page, totalPages)

		offset := (page - 1) * UsersPerPage
		err = db.
			Preload("Coalition").
			Preload("Title").
			Preload("Campus").
			Offset(offset).
			Limit(UsersPerPage).
			Order(sorting + " DESC").
			Scopes(database.OnlyRealUsers()).
			Scopes(database.WithCampus(campus)).
			Scopes(database.WithPromo(promo)).
			Find(&users).Error

		if err != nil {
			internalServerError(w, err)
			return
		}

		activeCampusId, _ := strconv.Atoi(campus)
		templates.Leaderboard(users,
			promos, campuses, activeCampusId,
			r.URL, page, totalPages, shownFields,
			offset).Render(r.Context(), w)
	})
}
