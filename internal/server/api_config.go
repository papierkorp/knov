package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get current configuration
// @Tags config
// @Produce json,html
// @Router /api/config [get]
func handleAPIGetConfig(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	userSettings := configmanager.GetUserSettings()

	config := struct {
		App  configmanager.AppConfig    `json:"app"`
		User configmanager.UserSettings `json:"user"`
	}{
		App:  appConfig,
		User: userSettings,
	}

	html := render.RenderConfigDisplay(userSettings, appConfig)
	writeResponse(w, r, config, html)
}

// @Summary Get current data path as input field
// @Tags config
// @Produce html
// @Router /api/config/datapath [get]
func handleAPIGetCurrentDataPath(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	dataPath := appConfig.DataPath

	html := render.RenderInputField("text", "dataPath", "data-path", dataPath, translation.SprintfForRequest(configmanager.GetLanguage(), "/path/to/data"), true)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Set language
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/language [post]
func handleAPISetLanguage(w http.ResponseWriter, r *http.Request) {
	lang := r.FormValue("language")

	logging.LogDebug("language set to: %s", lang)

	if lang != "" {
		configmanager.SetLanguage(lang)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// @Summary Update date display format
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param dateFormat formData string true "Date format: DD.MM.YYYY, YYYY-MM-DD, MM/DD/YYYY, or DD/MM/YYYY"
// @Produce json,html
// @Router /api/config/date-format [post]
func handleAPISetDateFormat(w http.ResponseWriter, r *http.Request) {
	dateFormat := r.FormValue("dateFormat")

	logging.LogDebug("date format set to: %s", dateFormat)

	if dateFormat != "" {
		configmanager.SetDateFormat(dateFormat)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// @Summary Get git remote URL
// @Tags config
// @Produce json,html
// @Success 200 {object} string
// @Router /api/config/repository [get]
func handleAPIGetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	repositoryURL := appConfig.GitRemote

	html := render.RenderInputField("text", "repositoryURL", "git-url", repositoryURL, translation.SprintfForRequest(configmanager.GetLanguage(), "https://github.com/user/repo.git or git@github.com:user/repo.git"), false)
	writeResponse(w, r, repositoryURL, html)
}

// @Summary Update git remote URL
// @Description updates git remote url in .env file
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param repositoryURL formData string true "remote repository url"
// @Produce json,html
// @Success 200 {string} string "saved"
// @Router /api/config/repository [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	repositoryURL := r.FormValue("repositoryURL")

	if err := configmanager.UpdateEnvFile("KNOV_GIT_REMOTE", repositoryURL); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save"), http.StatusInternalServerError)
		return
	}

	if err := git.EnsureRemote(); err != nil {
		logging.LogWarning("failed to configure git remote: %v", err)
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "git remote saved"))
	writeResponse(w, r, "saved", "")
}

// @Summary Restart application
// @Description Restarts the application (requires process manager like systemd or docker)
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {string} string "restarting"
// @Router /api/system/restart [post]
func handleAPIRestartApp(w http.ResponseWriter, r *http.Request) {
	logging.LogInfo("application restart requested")

	data := "restarting"
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "restarting application..."))
	writeResponse(w, r, data, "")

	// give response time to send
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

// @Summary Update data path
// @Description updates data path in .env file (requires restart)
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param dataPath formData string true "data path"
// @Produce json,html
// @Success 200 {string} string "saved"
// @Router /api/config/datapath [post]
func handleAPISetDataPath(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	dataPath := r.FormValue("dataPath")

	if dataPath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "data path cannot be empty"), http.StatusBadRequest)
		return
	}

	if err := configmanager.UpdateEnvFile("KNOV_DATA_PATH", dataPath); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save"), http.StatusInternalServerError)
		return
	}

	data := "saved"
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "data path saved. restart required."))
	writeResponse(w, r, data, "")
}

// @Summary Get available languages
// @Tags config
// @Produce json,html
// @Router /api/config/languages [get]
func handleAPIGetLanguages(w http.ResponseWriter, r *http.Request) {
	languages := configmanager.GetAvailableLanguages()
	currentLang := configmanager.GetLanguage()

	options := render.GetLanguageOptions()
	html := render.RenderSelectOptions(options, currentLang)
	writeResponse(w, r, languages, html)
}

// @Summary Update media upload size limit
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param maxUploadSizeMB formData int true "Maximum upload size in MB"
// @Produce json,html
// @Router /api/config/media/upload-size [post]
func handleAPIUpdateMediaUploadSize(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	sizeStr := r.FormValue("maxUploadSizeMB")
	if sizeStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing upload size"), http.StatusBadRequest)
		return
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 || size > 100 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid upload size"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.MaxUploadSizeMB = size

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated media upload size to %d MB", size)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "upload size updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update allowed MIME types
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param allowedMimeTypes formData string true "Comma-separated list of allowed MIME types"
// @Produce json,html
// @Router /api/config/media/mime-types [post]
func handleAPIUpdateMediaMimeTypes(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	mimeTypesStr := r.FormValue("allowedMimeTypes")
	if mimeTypesStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing mime types"), http.StatusBadRequest)
		return
	}

	// parse comma-separated list
	var mimeTypes []string
	for _, mimeType := range strings.Split(mimeTypesStr, ",") {
		trimmed := strings.TrimSpace(mimeType)
		if trimmed != "" {
			mimeTypes = append(mimeTypes, trimmed)
		}
	}

	if len(mimeTypes) == 0 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "at least one mime type required"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.AllowedMimeTypes = mimeTypes

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated allowed mime types: %v", mimeTypes)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "mime types updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update section edit include subheaders setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param sectionEditIncludeSubheaders formData bool true "Whether section editing should include subheaders"
// @Produce json,html
// @Router /api/config/section-edit-subheaders [post]
func handleAPIUpdateSectionEditSubheaders(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	includeSubheaders := r.FormValue("sectionEditIncludeSubheaders") == "true"

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.SectionEditIncludeSubheaders = includeSubheaders

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated section edit include subheaders to: %t", includeSubheaders)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "section edit setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update code block wrap setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param codeBlockWrap formData bool true "Whether code blocks should wrap long lines"
// @Produce json,html
// @Router /api/config/code-block-wrap [post]
func handleAPIUpdateCodeBlockWrap(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), "")
		return
	}

	wrap := r.FormValue("codeBlockWrap") == "true"

	userSettings := configmanager.GetUserSettings()
	userSettings.CodeBlockWrap = wrap
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated code block wrap to: %t", wrap)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "code block wrap setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update default preview size
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param defaultPreviewSize formData int true "Default preview size in pixels"
// @Produce json,html
// @Router /api/config/media/default-preview-size [post]
func handleAPIUpdateDefaultPreviewSize(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	sizeStr := r.FormValue("defaultPreviewSize")
	if sizeStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing preview size"), http.StatusBadRequest)
		return
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 50 || size > 1000 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid preview size"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.DefaultPreviewSize = size
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated default preview size to %d pixels", size)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "preview size updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update preview display mode
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param displayMode formData string true "Display mode: left, center, right, inline"
// @Produce json,html
// @Router /api/config/media/display-mode [post]
func handleAPIUpdateDisplayMode(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	displayMode := r.FormValue("displayMode")
	if displayMode == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing display mode"), http.StatusBadRequest)
		return
	}

	validModes := []string{"left", "center", "right", "inline"}
	isValid := false
	for _, mode := range validModes {
		if displayMode == mode {
			isValid = true
			break
		}
	}
	if !isValid {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid display mode"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.DisplayMode = displayMode
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated display mode to: %s", displayMode)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "display mode updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update preview border style
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param borderStyle formData string true "Border style: none, simple, rounded, shadow"
// @Produce json,html
// @Router /api/config/media/border-style [post]
func handleAPIUpdateBorderStyle(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	borderStyle := r.FormValue("borderStyle")
	if borderStyle == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing border style"), http.StatusBadRequest)
		return
	}

	validStyles := []string{"none", "simple", "rounded", "shadow"}
	isValid := false
	for _, style := range validStyles {
		if borderStyle == style {
			isValid = true
			break
		}
	}
	if !isValid {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid border style"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.BorderStyle = borderStyle
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated border style to: %s", borderStyle)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "border style updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update show caption setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param showCaption formData bool true "Whether to show captions"
// @Produce json,html
// @Router /api/config/media/show-caption [post]
func handleAPIUpdateShowCaption(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	showCaption := r.FormValue("showCaption") == "true"

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.ShowCaption = showCaption
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated show caption to: %t", showCaption)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "caption setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update click to enlarge setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param clickToEnlarge formData bool true "Whether previews are clickable"
// @Produce json,html
// @Router /api/config/media/click-to-enlarge [post]
func handleAPIUpdateClickToEnlarge(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	clickToEnlarge := r.FormValue("clickToEnlarge") == "true"

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.ClickToEnlarge = clickToEnlarge
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated click to enlarge to: %t", clickToEnlarge)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "click setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update preview enabled setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param enablePreviews formData bool true "Whether previews are enabled"
// @Produce json,html
// @Router /api/config/media/enable-previews [post]
func handleAPIUpdateEnablePreviews(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	enablePreviews := r.FormValue("enablePreviews") == "true"

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.EnablePreviews = enablePreviews
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated enable previews to: %t", enablePreviews)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "preview setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide markdown files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideMarkdown formData bool true "Whether to hide markdown files"
// @Produce json,html
// @Router /api/config/file-types/hide-markdown [post]
func handleAPIUpdateHideMarkdown(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hideMarkdown := r.FormValue("hideMarkdown") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_MARKDOWN", fmt.Sprintf("%t", hideMarkdown)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide markdown to: %t", hideMarkdown)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "markdown visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide text files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideText formData bool true "Whether to hide text files"
// @Produce json,html
// @Router /api/config/file-types/hide-text [post]
func handleAPIUpdateHideText(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hideText := r.FormValue("hideText") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_TEXT", fmt.Sprintf("%t", hideText)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide text to: %t", hideText)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "text visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide list files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideList formData bool true "Whether to hide list files"
// @Produce json,html
// @Router /api/config/file-types/hide-list [post]
func handleAPIUpdateHideList(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hideList := r.FormValue("hideList") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_LIST", fmt.Sprintf("%t", hideList)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide list to: %t", hideList)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "list visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide todo files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideTodo formData bool true "Whether to hide todo files"
// @Produce json,html
// @Router /api/config/file-types/hide-todo [post]
func handleAPIUpdateHideTodo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hideTodo := r.FormValue("hideTodo") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_TODO", fmt.Sprintf("%t", hideTodo)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide todo to: %t", hideTodo)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "todo visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide filter files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideFilter formData bool true "Whether to hide filter files"
// @Produce json,html
// @Router /api/config/file-types/hide-filter [post]
func handleAPIUpdateHideFilter(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hideFilter := r.FormValue("hideFilter") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_FILTER", fmt.Sprintf("%t", hideFilter)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide filter to: %t", hideFilter)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "filter visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide index files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideIndex formData bool true "Whether to hide index files"
// @Produce json,html
// @Router /api/config/file-types/hide-index [post]
func handleAPIUpdateHideIndex(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hideIndex := r.FormValue("hideIndex") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_INDEX", fmt.Sprintf("%t", hideIndex)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide index to: %t", hideIndex)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "index visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide image files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideImage formData bool true "Whether to hide image files"
// @Produce json,html
// @Router /api/config/file-types/hide-image [post]
func handleAPIUpdateHideImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hideImage := r.FormValue("hideImage") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_IMAGE", fmt.Sprintf("%t", hideImage)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide image to: %t", hideImage)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "image visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide video files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideVideo formData bool true "Whether to hide video files"
// @Produce json,html
// @Router /api/config/file-types/hide-video [post]
func handleAPIUpdateHideVideo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hideVideo := r.FormValue("hideVideo") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_VIDEO", fmt.Sprintf("%t", hideVideo)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide video to: %t", hideVideo)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "video visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide pdf files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hidePDF formData bool true "Whether to hide pdf files"
// @Produce json,html
// @Router /api/config/file-types/hide-pdf [post]
func handleAPIUpdateHidePDF(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hidePDF := r.FormValue("hidePDF") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_PDF", fmt.Sprintf("%t", hidePDF)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide pdf to: %t", hidePDF)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "pdf visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update table page size
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param pageSize formData int true "Rows per page (5-200)"
// @Produce json,html
// @Router /api/config/table/page-size [post]
func handleAPIUpdateTablePageSize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	size, err := strconv.Atoi(r.FormValue("pageSize"))
	if err != nil || size < 5 || size > 200 {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid page size"))
		return
	}
	us := configmanager.GetUserSettings()
	us.TableSettings.PageSize = size
	configmanager.SetUserSettings(us)
	logging.LogInfo("updated table page size to %d", size)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "table page size updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update table show search setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param showSearch formData bool true "Whether to show the search input"
// @Produce json,html
// @Router /api/config/table/show-search [post]
func handleAPIUpdateTableShowSearch(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.TableSettings.ShowSearch = r.FormValue("showSearch") == "true"
	configmanager.SetUserSettings(us)
	logging.LogInfo("updated table show search to: %t", us.TableSettings.ShowSearch)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "table search setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update table show info setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param showInfo formData bool true "Whether to show the row count info line"
// @Produce json,html
// @Router /api/config/table/show-info [post]
func handleAPIUpdateTableShowInfo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.TableSettings.ShowInfo = r.FormValue("showInfo") == "true"
	configmanager.SetUserSettings(us)
	logging.LogInfo("updated table show info to: %t", us.TableSettings.ShowInfo)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "table info setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update table show paging setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param showPaging formData bool true "Whether to show pagination buttons"
// @Produce json,html
// @Router /api/config/table/show-paging [post]
func handleAPIUpdateTableShowPaging(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.TableSettings.ShowPaging = r.FormValue("showPaging") == "true"
	configmanager.SetUserSettings(us)
	logging.LogInfo("updated table show paging to: %t", us.TableSettings.ShowPaging)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "table paging setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update ToastUI initial view mode
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param toastuiInitialView formData string true "Initial edit type: markdown or wysiwyg"
// @Produce json,html
// @Router /api/config/editor/toastui-view [post]
func handleAPIUpdateToastuiInitialView(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	view := r.FormValue("toastuiInitialView")
	if view != "markdown" && view != "wysiwyg" {
		view = "markdown"
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.ToastuiInitialView = view
	configmanager.SetUserSettings(us)
	logging.LogInfo("updated toastui initial view to: %s", view)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update ToastUI preview style
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param toastuiPreviewStyle formData string true "Preview style: tab or vertical"
// @Produce json,html
// @Router /api/config/editor/toastui-preview [post]
func handleAPIUpdateToastuiPreviewStyle(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	style := r.FormValue("toastuiPreviewStyle")
	if style != "tab" && style != "vertical" {
		style = "tab"
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.ToastuiPreviewStyle = style
	configmanager.SetUserSettings(us)
	logging.LogInfo("updated toastui preview style to: %s", style)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update CodeMirror vim mode
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param codeMirrorVimMode formData bool true "Whether to enable vim keybindings"
// @Produce json,html
// @Router /api/config/editor/vim-mode [post]
func handleAPIUpdateCodeMirrorVimMode(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.CodeMirrorVimMode = r.FormValue("codeMirrorVimMode") == "true"
	configmanager.SetUserSettings(us)
	logging.LogInfo("updated codemirror vim mode to: %t", us.EditorSettings.CodeMirrorVimMode)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update CodeMirror line numbers
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param codeMirrorLineNumbers formData bool true "Whether to show line numbers"
// @Produce json,html
// @Router /api/config/editor/codemirror-line-numbers [post]
func handleAPIUpdateCodeMirrorLineNumbers(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.CodeMirrorLineNumbers = r.FormValue("codeMirrorLineNumbers") == "true"
	configmanager.SetUserSettings(us)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update CodeMirror relative line numbers
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param codeMirrorRelativeLineNumbers formData bool true "Whether to show relative line numbers"
// @Produce json,html
// @Router /api/config/editor/codemirror-relative-line-numbers [post]
func handleAPIUpdateCodeMirrorRelativeLineNumbers(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.CodeMirrorRelativeLineNumbers = r.FormValue("codeMirrorRelativeLineNumbers") == "true"
	configmanager.SetUserSettings(us)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update CodeMirror fold gutter
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param codeMirrorFoldGutter formData bool true "Whether to show the fold gutter"
// @Produce json,html
// @Router /api/config/editor/codemirror-fold-gutter [post]
func handleAPIUpdateCodeMirrorFoldGutter(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.CodeMirrorFoldGutter = r.FormValue("codeMirrorFoldGutter") == "true"
	configmanager.SetUserSettings(us)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update CodeMirror bracket matching
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param codeMirrorBracketMatching formData bool true "Whether to highlight matching brackets"
// @Produce json,html
// @Router /api/config/editor/codemirror-bracket-matching [post]
func handleAPIUpdateCodeMirrorBracketMatching(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.CodeMirrorBracketMatching = r.FormValue("codeMirrorBracketMatching") == "true"
	configmanager.SetUserSettings(us)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update CodeMirror auto brackets
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param codeMirrorAutoBrackets formData bool true "Whether to auto-close brackets"
// @Produce json,html
// @Router /api/config/editor/codemirror-auto-brackets [post]
func handleAPIUpdateCodeMirrorAutoBrackets(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.CodeMirrorAutoBrackets = r.FormValue("codeMirrorAutoBrackets") == "true"
	configmanager.SetUserSettings(us)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update CodeMirror highlight selection matches
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param codeMirrorHighlightSelection formData bool true "Whether to highlight all occurrences of the selected text"
// @Produce json,html
// @Router /api/config/editor/codemirror-highlight-selection [post]
func handleAPIUpdateCodeMirrorHighlightSelection(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.CodeMirrorHighlightSelection = r.FormValue("codeMirrorHighlightSelection") == "true"
	configmanager.SetUserSettings(us)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update CodeMirror highlight selection whole word mode
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param codeMirrorHighlightSelectionWholeWord formData bool true "Whether to only highlight whole-word matches"
// @Produce json,html
// @Router /api/config/editor/codemirror-highlight-selection-whole-word [post]
func handleAPIUpdateCodeMirrorHighlightSelectionWholeWord(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.CodeMirrorHighlightSelectionWholeWord = r.FormValue("codeMirrorHighlightSelectionWholeWord") == "true"
	configmanager.SetUserSettings(us)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update ToastUI toolbar visibility
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param toastuiShowToolbar formData bool true "Whether to show the formatting toolbar"
// @Produce json,html
// @Router /api/config/editor/toastui-toolbar [post]
func handleAPIUpdateToastuiShowToolbar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.ToastuiShowToolbar = r.FormValue("toastuiShowToolbar") == "true"
	configmanager.SetUserSettings(us)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update ToastUI mode switch bar visibility
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param toastuiShowModeSwitch formData bool true "Whether to show the markdown/WYSIWYG switch tab"
// @Produce json,html
// @Router /api/config/editor/toastui-modeswitch [post]
func handleAPIUpdateToastuiShowModeSwitch(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.ToastuiShowModeSwitch = r.FormValue("toastuiShowModeSwitch") == "true"
	configmanager.SetUserSettings(us)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update spell check setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param spellCheck formData bool true "Whether to enable spell checking in editors"
// @Produce json,html
// @Router /api/config/editor/spell-check [post]
func handleAPIUpdateSpellCheck(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.SpellCheck = r.FormValue("spellCheck") == "true"
	configmanager.SetUserSettings(us)
	logging.LogInfo("updated spell check to: %t", us.EditorSettings.SpellCheck)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Router /api/config/editor/wiki-link-cursor-end [post]
func handleAPIUpdateWikiLinkCursorEnd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeResponse(w, r, nil, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"))
		return
	}
	us := configmanager.GetUserSettings()
	us.EditorSettings.WikiLinkCursorEnd = r.FormValue("wikiLinkCursorEnd") == "true"
	configmanager.SetUserSettings(us)
	logging.LogInfo("updated wiki link cursor end to: %t", us.EditorSettings.WikiLinkCursorEnd)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "editor setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Upload custom favicon
// @Description Uploads a custom favicon (ico, png, or svg) stored in storage/favicon
// @Tags config
// @Accept multipart/form-data
// @Param file formData file true "Favicon file (.ico, .png, or .svg)"
// @Produce html
// @Success 200 {string} string "favicon uploaded"
// @Failure 400 {string} string "invalid file"
// @Failure 500 {string} string "upload failed"
// @Router /api/config/favicon [post]
func handleAPIUploadFavicon(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		writeResponse(w, r, nil, "")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "no file uploaded"))
		writeResponse(w, r, nil, "")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".ico" && ext != ".png" && ext != ".svg" {
		w.WriteHeader(http.StatusBadRequest)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "only .ico, .png and .svg files are allowed"))
		writeResponse(w, r, nil, "")
		return
	}

	faviconDir := filepath.Join(configmanager.GetAppConfig().StoragePath, "favicon")
	if err := os.MkdirAll(faviconDir, 0755); err != nil {
		logging.LogError("favicon upload: failed to create directory: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"))
		writeResponse(w, r, nil, "")
		return
	}

	destPath := filepath.Join(faviconDir, "favicon"+ext)
	data, err := io.ReadAll(file)
	if err != nil {
		logging.LogError("favicon upload: failed to read file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
		writeResponse(w, r, nil, "")
		return
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		logging.LogError("favicon upload: failed to write file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"))
		writeResponse(w, r, nil, "")
		return
	}

	// store the chosen extension in settings so the favicon route knows which file to serve
	userSettings := configmanager.GetUserSettings()
	userSettings.CustomFaviconExt = ext
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("favicon uploaded: %s", destPath)
	w.Header().Set("HX-Trigger", "faviconChanged")
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "favicon uploaded"))
	writeResponse(w, r, nil, "")
}

// @Summary Delete custom favicon
// @Description Removes the custom favicon and reverts to the default
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce html
// @Success 200 {string} string "favicon removed"
// @Failure 500 {string} string "failed to remove"
// @Router /api/config/favicon [delete]
func handleAPIDeleteFavicon(w http.ResponseWriter, r *http.Request) {
	userSettings := configmanager.GetUserSettings()
	ext := userSettings.CustomFaviconExt
	if ext == "" {
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "no custom favicon set"))
		writeResponse(w, r, nil, "")
		return
	}

	destPath := filepath.Join(configmanager.GetAppConfig().StoragePath, "favicon", "favicon"+ext)
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		logging.LogError("favicon delete: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to remove favicon"))
		writeResponse(w, r, nil, "")
		return
	}

	userSettings.CustomFaviconExt = ""
	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("custom favicon removed")
	w.Header().Set("HX-Trigger", "faviconChanged")
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "custom favicon removed"))
	writeResponse(w, r, nil, "")
}

// @Summary Update hide office document files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideOfficeDocuments formData bool true "Whether to hide office document files"
// @Produce json,html
// @Router /api/config/file-types/hide-office-documents [post]
func handleAPIUpdateHideOfficeDocuments(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hide := r.FormValue("hideOfficeDocuments") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_OFFICE_DOCUMENTS", fmt.Sprintf("%t", hide)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide office documents to: %t", hide)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "office document visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide archive files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideArchives formData bool true "Whether to hide archive files"
// @Produce json,html
// @Router /api/config/file-types/hide-archives [post]
func handleAPIUpdateHideArchives(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hide := r.FormValue("hideArchives") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_ARCHIVES", fmt.Sprintf("%t", hide)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide archives to: %t", hide)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "archive visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide executable files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideExecutables formData bool true "Whether to hide executable files"
// @Produce json,html
// @Router /api/config/file-types/hide-executables [post]
func handleAPIUpdateHideExecutables(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hide := r.FormValue("hideExecutables") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_EXECUTABLES", fmt.Sprintf("%t", hide)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide executables to: %t", hide)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "executable visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update hide script files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param hideScripts formData bool true "Whether to hide script files"
// @Produce json,html
// @Router /api/config/file-types/hide-scripts [post]
func handleAPIUpdateHideScripts(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	hide := r.FormValue("hideScripts") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_HIDE_SCRIPTS", fmt.Sprintf("%t", hide)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated hide scripts to: %t", hide)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "script visibility updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update show hidden files setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param showHiddenFiles formData bool true "Whether to show hidden files"
// @Produce json,html
// @Router /api/config/file-types/show-hidden [post]
func handleAPIUpdateShowHiddenFiles(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	show := r.FormValue("showHiddenFiles") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_SHOW_HIDDEN_FILES", fmt.Sprintf("%t", show)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated show hidden files to: %t", show)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "hidden files setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update home dashboard setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param homeDashboard formData string false "Dashboard ID to use as home page (empty for default)"
// @Produce json,html
// @Router /api/config/home-dashboard [post]
func handleAPIUpdateHomeDashboard(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	id := r.FormValue("homeDashboard")
	if err := configmanager.UpdateEnvFile("KNOV_HOME_DASHBOARD", id); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated home dashboard to: %s", id)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "home dashboard updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update use extension for todo files
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param useExtensionTodo formData bool true "Whether to use .todo extension"
// @Produce json,html
// @Router /api/config/extensions/todo [post]
func handleAPIUpdateUseExtensionTodo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	use := r.FormValue("useExtensionTodo") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_USE_EXTENSION_TODO", fmt.Sprintf("%t", use)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated use extension todo to: %t", use)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "todo extension setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update use extension for list files
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param useExtensionList formData bool true "Whether to use .list extension"
// @Produce json,html
// @Router /api/config/extensions/list [post]
func handleAPIUpdateUseExtensionList(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	use := r.FormValue("useExtensionList") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_USE_EXTENSION_LIST", fmt.Sprintf("%t", use)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated use extension list to: %t", use)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "list extension setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update use extension for index files
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param useExtensionIndex formData bool true "Whether to use .index extension"
// @Produce json,html
// @Router /api/config/extensions/index [post]
func handleAPIUpdateUseExtensionIndex(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	use := r.FormValue("useExtensionIndex") == "true"
	if err := configmanager.UpdateEnvFile("KNOV_USE_EXTENSION_INDEX", fmt.Sprintf("%t", use)); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated use extension index to: %t", use)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "index extension setting updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Update log level setting
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param logLevel formData string true "Log level (debug, info, warning, error)"
// @Produce json,html
// @Router /api/config/log-level [post]
func handleAPIUpdateLogLevel(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}
	level := r.FormValue("logLevel")
	if err := configmanager.UpdateEnvFile("KNOV_LOG_LEVEL", level); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"), http.StatusInternalServerError)
		return
	}
	logging.LogInfo("updated log level to: %s", level)
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "log level updated"))
	writeResponse(w, r, "saved", "")
}

// @Summary Export user settings as JSON
// @Description Downloads the current user settings as a JSON file
// @Tags config
// @Produce application/json
// @Router /api/config/export [get]
func handleAPIExportSettings(w http.ResponseWriter, r *http.Request) {
	settings := configmanager.GetUserSettings()
	data, err := json.Marshal(settings)
	if err != nil {
		logging.LogError("failed to marshal settings for export: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export settings"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"knov-settings.json\"")
	w.Write(data)
}

// @Summary Import user settings from JSON
// @Description Uploads and applies user settings from a JSON file
// @Tags config
// @Accept multipart/form-data
// @Param file formData file true "Settings JSON file"
// @Produce html
// @Router /api/config/import [post]
func handleAPIImportSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file"), http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusBadRequest)
		return
	}

	var settings configmanager.UserSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid settings file"), http.StatusBadRequest)
		return
	}

	configmanager.SetUserSettings(settings)
	logging.LogInfo("settings imported successfully")
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "settings imported successfully"))
	w.Header().Set("HX-Refresh", "true")
	writeResponse(w, r, map[string]string{"status": "imported"}, "")
}
