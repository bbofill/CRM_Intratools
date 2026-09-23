package module4workers

import (
	"io"
	"net/http"
	"time"
)

func serveAPI(w http.ResponseWriter, r *http.Request) {

	//req api key
	apiKey := r.FormValue("key")

	if verifyAPIKey(apiKey) {
		//set client

		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		if mC.Development {
			client = setInsecureRequest()
		}

		switch r.URL.Query().Get("action") {

		//ACCESS CONTROL TO WORKER DATA
		case "checkRole":
			handleCheckRole(apiKey, client, w, r)
		case "getUsersAccess":
			handleGetUsersAccess(apiKey, client, w, r)
		case "setUsersAccess":
			handleSetUsersAccess(apiKey, client, w, r)
		case "checkAccess":
			handleCheckAccess(apiKey, client, w, r)

		//MANAGE USERS
		case "allUneixUsersSummary":
			// returns only the fields needed to render UsersHub cards
			handleGetUneixUsersSummary(apiKey, client, w, r)
		case "get-options":
			//returns all form options to front-end
			handleGetOptions(apiKey, client, w, r)
		case "allUneixUsers":
			//returns all user info to users hub
			handleGetUneixUsers(apiKey, client, w, r)
		case "saveUser":
			//manages adding and editing users
			handleSaveUser(apiKey, client, w, r)

		//NEW HIRES
		case "getNewHires":
			handleGetNewHires(apiKey, client, w, r)
		case "newHiresRequestDesk":
			handleDeskTicket(apiKey, client, w, r)
		case "newHiresRequestAccount":
			handleAccountTicket(apiKey, client, w, r)
		case "newHiresAddToIntranet":
			handleAddToIntranet(apiKey, client, w, r)
		case "getNewHiresHistory":
			handleGetNewHiresHistory(apiKey, client, w, r)

			//MANAGE NOTIFICATIONS
		case "notification":
			handleCreateNotification(apiKey, client, w, r)
		case "get-users":
			handleGetUsers(apiKey, client, w)

			//MANAGE GROUPS
		case "allGroups":
			handleGetAllGroups(apiKey, client, w, r)
		case "getGroupMembers":
			handleGetGroupMembers(apiKey, client, w, r)
		case "saveGroup":
			handleSaveGroup(apiKey, client, w, r)
		case "saveGroupMember":
			handleSaveGroupMembers(apiKey, client, w, r)
		case "deleteGroupMember":
			handleDeleteGroupMembers(apiKey, client, w, r)
		case "addGroupMember":
			handleAddGroupMember(apiKey, client, w, r)
		case "getAvailableUsersForGroup":
			handleGetAvailableUsersForGroup(apiKey, client, w, r)

			//MANAGE TRAININGS
		case "allTrainings":
			handleGetAllTrainings(apiKey, client, w, r)
		case "saveTraining":
			handleSaveTraining(apiKey, client, w, r)
		case "deleteTraining":
			handleDeleteTraining(apiKey, client, w, r)
		case "getTrainingMembers":
			handleGetTrainingMembers(apiKey, client, w, r)
		case "saveTrainingMember":
			handleSaveTrainingMembers(apiKey, client, w, r)
		case "addTrainingMember":
			handleAddTrainingMember(apiKey, client, w, r)
		case "getAvailableUsers":
			handleGetAvailableUsers(apiKey, client, w, r)

			//MANAGE FUNDINGS
		case "allFundings":
			handleGetFundings(apiKey, client, w, r)
		case "saveFunding":
			handleSaveFunding(apiKey, client, w, r)
		case "deleteFunding":
			handleDeleteFunding(apiKey, client, w, r)

			//MANAGE SPINOFFS
		case "allSpinoffs":
			handleGetSpinoffs(apiKey, client, w, r)
		case "saveSpinoff":
			handleSaveSpinoff(apiKey, client, w, r)
		case "deleteSpinoff":
			handleDeleteSpinoff(apiKey, client, w, r)

			// MANAGE PROJECTS
		case "allProjects":
			handleGetProjects(apiKey, client, w, r)
		case "projectTypes":
			handleGetProjectTypes(apiKey, client, w, r)
		case "getProjectMembers":
			handleGetProjectMembers(apiKey, client, w, r)
		case "saveProject":
			handleSaveProject(apiKey, client, w, r)
		case "saveProjectMember":
			handleSaveProjectMembers(apiKey, client, w, r)
		case "deleteProjectMember":
			handleDeleteProjectMembers(apiKey, client, w, r)
		case "addProjectMember":
			handleAddProjectMember(apiKey, client, w, r)
		case "getAvailableUsersForProject":
			handleGetAvailableUsersForProject(apiKey, client, w, r)
		case "deleteProject":
			handleDeleteProject(apiKey, client, w, r)

		}

	} else {
		http.Error(w, "Forbidden", http.StatusForbidden)
		createLog("Invalid API key provided", 2, apiKey, nil, w)
		return
	}

}

func serveUpload(w http.ResponseWriter, r *http.Request) {
	apiKey := r.FormValue("key")

	if !verifyAPIKey(apiKey) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		createLog("Invalid API key provided", 2, apiKey, nil, w)
		return
	}

	if verifyAPIKey(apiKey) {
		req, _ := http.NewRequest(http.MethodGet, "https://localhost:"+mC.ServerPort+"/"+r.URL.Query().Get("file"), nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		if mC.Development {
			client = setInsecureRequest()
		}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Error haciendo forwarding", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
