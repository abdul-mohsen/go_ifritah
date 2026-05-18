package handlers

import (
	"net/http"

	"afrita/config"
	"afrita/helpers"

	"github.com/gorilla/mux"
)

func HandleSendInvoiceWhatsApp(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	id := mux.Vars(r)["id"]
	req, _ := http.NewRequest(http.MethodPost, config.BackendDomain+"/api/v2/bill/"+id+"/whatsapp", nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, "فشل الاتصال بخدمة واتساب")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "فشل إرسال الفاتورة عبر واتساب")
		return
	}

	helpers.WriteSuccessToast(w, "تم إرسال الفاتورة عبر واتساب")
	w.Header().Set("HX-Reswap", "none")
	w.WriteHeader(http.StatusOK)
}
