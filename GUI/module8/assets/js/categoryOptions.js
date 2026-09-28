  // ---------- Templates ----------
  export function sectionTemplate(id, title, innerHtml, prefix = "main", parentId = "details-accordion") {
    const safeId = `${prefix}-sec-${id}`;
    return `
      <div class="accordion-item" id="${safeId}" data-section="${id}">
        <h2 class="accordion-header" id="${safeId}-head">
          <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse"
                  data-bs-target="#${safeId}-body" aria-expanded="false" aria-controls="${safeId}-body">
            ${title}
          </button>
        </h2>
        <div id="${safeId}-body" class="accordion-collapse collapse" aria-labelledby="${safeId}-head"
            data-bs-parent="#${parentId}">
          <div class="accordion-body">
            ${innerHtml}
          </div>
        </div>
      </div>
    `;
  }

  export function travelFields() {
    return `
      <div class="row g-3">
        <div class="col-md-6">
          <label class="form-label">From where</label>
          <input type="text" class="form-control" name="travel_from_where" placeholder="City / airport / address">
        </div>
        <div class="col-md-6">
          <label class="form-label">To where</label>
          <input type="text" class="form-control" name="travel_to_where" placeholder="City / airport / address">
        </div>
        <div class="col-md-6">
          <label class="form-label">Destination institution</label>
          <input type="text" class="form-control" name="institution" placeholder="Institution / university / center...">
        </div>

        <div class="col-md-6">
          <label class="form-label">Travel area</label>
          <select class="form-select" name="travel_area">
            <option value="">Select...</option>
            <option value="spain">Inside Spanish territory</option>
            <option value="eu">European Union</option>
            <option value="non_eu">Outside the European Union (including the United Kingdom)</option>
          </select>
        </div>
        <div class="col-md-6">
          <label class="form-label">From this day</label>
          <input type="date" class="form-control" name="travel_from_day">
        </div>
        <div class="col-md-6">
          <label class="form-label">Until this day</label>
          <input type="date" class="form-control" name="travel_until_day">
        </div>

        <div class="col-12">
          <label class="form-label">Purpose</label>
          <input type="text" class="form-control" name="travel_purpose" placeholder="e.g., conference, project meeting...">
        </div>

        <div class="col-md-6">
          <label class="form-label">Project</label>
          <select class="form-select" name="travel_project">
            <option value="">Select...</option>
          </select>
        </div>

        <div class="col-md-6 d-none" data-ip-wrapper="travel">
          <label class="form-label">Send to responsible (PI, supervisor...)</label>
          <select class="form-select" name="travel_ip">
            <option value="">Select responsible...</option>
          </select>
        </div>

        <div class="col-md-6">
          <label class="form-label">Luggage type</label>
          <select class="form-select" name="travel_luggage_type">
            <option value="">Select...</option>
            <option value="none">No luggage</option>
            <option value="hand">Hand luggage only</option>
            <option value="checked">Checked luggage only</option>
            <option value="hand_checked">Hand luggage + checked luggage</option>
          </select>
        </div>

        <div class="col-md-6 d-none" data-travel-checked-kg-wrapper>
          <label class="form-label">Checked luggage weight</label>
          <select class="form-select" name="travel_checked_kg">
            <option value="">Select...</option>
            <option value="15">15 kg</option>
            <option value="20">20 kg</option>
            <option value="23">23 kg</option>
            <option value="25">25 kg</option>
            <option value="30">30 kg</option>
          </select>
        </div>

        <div class="col-md-6">
          <label class="form-label">Seat preference</label>
          <select class="form-select" name="travel_seat_preference">
            <option value="">Select...</option>
            <option value="na">Random</option>
            <option value="window">Window</option>
            <option value="middle">Middle</option>
            <option value="aisle">Aisle</option>
          </select>
        </div>

        <div class="col-md-6">
          <label class="form-label">Time preference</label>
          <select class="form-select" name="travel_time_preference">
            <option value="">Select...</option>
            <option value="na">No preference</option>
            <option value="morning">Morning</option>
            <option value="afternoon">Afternoon</option>
          </select>
        </div>

        <div class="col-12">
          <div class="alert alert-info mb-0" role="alert">
            We will take your travel preferences into consideration whenever possible, subject to availability and budget constraints.
            Requests to travel by private car must be reasonable and are subject to approval depending on the cost and duration of the trip compared with available alternatives such as plane or train.
          </div>
        </div>

        <div class="col-12">
          <label class="form-label optional">Observations</label>
          <textarea
            class="form-control"
            name="travel_observations"
            rows="3"
            placeholder="Any additional notes, required health insurance, specific flight/airport preferences, transportation needed upon arrival, etc. \nIf you plan to travel with your own car, please mention it here and include the vehicle license plate number."></textarea>
          </div>

        <div>
            <label class="form-label optional">Invite</label>
            <input
              class="form-control"
              type="file"
              name="travel_preferences_files"
              accept="image/*,application/pdf"
              multiple
              data-max-files="3"
            >
            <div class="form-text">Upload up to 3 images or PDFs. Maximum file size: 10MB</div>
            <div class="selected-files-list mt-2" data-files-list="travel_preferences_files"></div>
        </div>
        <div data-server-files="travel" class="mt-2"></div>
      </div>
    `;
  }
  export function registrationFields() {
    return `
    <div class="row g-3">

        <div class="col-12">
          <label class="form-label">Event</label>
          <input type="text" class="form-control" name="registration_event" placeholder="e.g., conference, project meeting...">
        </div>
        <div class="col-md-6">
          <label class="form-label">Event start date</label>
          <input type="date" class="form-control" name="registration_from_day">
        </div>
        <div class="col-md-6">
          <label class="form-label">Event end date</label>
          <input type="date" class="form-control" name="registration_until_day">
        </div>


      <div id="invoice-wrapper">
      <div class="mb-2">
        <label class="form-label">Do you have an invoice or a payment document?</label>
        <div class="d-flex gap-4">
          <div class="form-check">
            <input class="form-check-input" type="radio" name="registration_registered" id="reg-yes" value="yes">
            <label class="form-check-label" for="reg-yes">Yes</label>
          </div>
          <div class="form-check">
            <input class="form-check-input" type="radio" name="registration_registered" id="reg-no" value="no">
            <label class="form-check-label" for="reg-no">No</label>
          </div>
        </div>
      </div>

        <div id="registration-invoice-wrapper" class="mt-3 d-none">
        <label class="form-label">Select the type of document</label>
            <div class="d-flex gap-4">
              <div class="form-check">
                <input class="form-check-input" type="radio" name="registration_payment" id="pay-yes" value="invoice">
                <label class="form-check-label" for="pay-yes">Invoice</label>
              </div>
              <div class="form-check">
                <input class="form-check-input" type="radio" name="registration_payment" id="pay-no" value="payment">
                <label class="form-check-label" for="pay-no">Payment Document</label>
              </div>
            </div>
            <p class="text-muted mt-2">Please remember that the invoice must be issued with the CRM tax details.</p>
            <p class="text-muted mt-2">If you attach a payment document, the request will remain pending until you provide the invoice. Please make sure to upload it in the <em>Sent requests</em> section as soon as possible.</p>
            <div>
              <label class="form-label">Please attach the invoice or payment document.</label>
              <input
                class="form-control"
                type="file"
                name="registration_invoice"
                accept="image/*,application/pdf"
                multiple
                data-max-files="3"
              >
              <div class="form-text">Upload up to 3 images or PDFs. Maximum file size: 10MB</div>
              <div class="selected-files-list mt-2" data-files-list="registration_invoice"></div>
            </div>
        </div>
        <div data-server-files="registration" class="mt-2"></div>

        <div id="registration-warning" class="mt-3 d-none">
            <div class="alert alert-warning">
                A meeting with the Projects and Accounting department will be required to process your request. Please be prepared to provide any additional information they may require.
            </div>
        </div>
        </div>
        <div class="col-md-12">
          <label class="form-label">Project</label>
          <select class="form-select" name="registration_project">
            <option value="">Select...</option>
          </select>
        </div>

        <div class="col-md-12 d-none" data-ip-wrapper="registration">
          <label class="form-label">Send to responsible (PI, supervisor...)</label>
          <select class="form-select" name="registration_ip">
            <option value="">Select responsible...</option>
          </select>
        </div>
        <div class="col-12">
          <label class="form-label optional">Observations</label>
          <textarea class="form-control" name="registration_observations" rows="3" placeholder="If a meeting is required, please provide details of availability. Any additional information..."></textarea>
        </div>
      </div>
    `;
  }

  export function accommodationFields() {
    return `
      <div class="row g-3">
        <div class="col-md-4">
          <label class="form-label">Where</label>
          <input type="text" class="form-control" name="accommodation_where" placeholder="City / address">
        </div>

        <div class="col-md-4">
          <label class="form-label">From this day</label>
          <input type="date" class="form-control" name="accommodation_from_day">
        </div>

        <div class="col-md-4">
          <label class="form-label">Until this day</label>
          <input type="date" class="form-control" name="accommodation_until_day">
        </div>

        <div class="col-12">
          <label class="form-label">Purpose</label>
          <input type="text" class="form-control" name="accommodation_purpose" placeholder="e.g., conference, project meeting...">
        </div>

        <div class="col-md-6">
          <label class="form-label">Project</label>
          <select class="form-select" name="accommodation_project">
            <option value="">Select...</option>
          </select>
        </div>

        <div class="col-md-6 d-none" data-ip-wrapper="accommodation">
          <label class="form-label">Send to responsible (PI, supervisor...)</label>
          <select class="form-select" name="accommodation_ip">
            <option value="">Select responsible...</option>
          </select>
        </div>

        <div class="col-12">
          <label class="form-label optional">Observations</label>
          <textarea class="form-control" name="accommodation_observations" rows="3" placeholder="Any additional notes..."></textarea>
        </div>
      </div>
    `;
  }

  export function equipmentFields() {
    return `
      <div class="row g-3">
        <div class="col-md-6">
          <label class="form-label">Category</label>
          <select class="form-select" name="equipment_category" id="equipment-category">
            <option value="">Select...</option>
            <option value="screen">Screen</option>
            <option value="laptop">Laptop</option>
            <option value="pc">PC</option>
            <option value="peripheral">Peripheral</option>
            <option value="other">Other</option>
          </select>
        </div>

        <div class="col-md-12 d-none" id="equipment-other-wrapper">
        <div class="row g-3">
            <div class="col-md-6">
            <label class="form-label">Specify other</label>
            <input
                type="text"
                class="form-control"
                name="equipment_other_text"
                placeholder="Please specify..."
            >
            </div>

            <div class="col-md-6">
            <label class="form-label">Specify price range</label>
            <select class="form-select" name="equipment_price_range">
                <option value="">Select...</option>
                <option value="1">0 - 5.000€</option>
                <option value="2">5.000 - 15.000€</option>
                <option value="3">15.000 - 50.000€</option>
                <option value="4">More than 50.000€</option>
            </select>
            </div>

            <div class="col-12 d-none" data-procurement-warning="equipment">
              <div class="alert alert-warning mb-0" role="alert">
                Requests in this price range may require the initiation of a public procurement procedure. Management will contact you in the next few days.
              </div>
            </div>
        </div>
        </div>

        <div class="col-12">
          <label class="form-label">Description</label>
          <textarea class="form-control" name="equipment_description" rows="3" placeholder="Describe the equipment request and/or attach a product link."></textarea>
        </div>

        <div class="col-md-12">
          <label class="form-label">Project</label>
          <select class="form-select" name="equipment_project">
            <option value="">Select...</option>
          </select>
        </div>

        <div class="col-md-12 d-none" data-ip-wrapper="equipment">
          <label class="form-label">Send to responsible (PI, supervisor...)</label>
          <select class="form-select" name="equipment_ip">
            <option value="">Select responsible...</option>
          </select>
        </div>
      </div>
    `;
  }

  export function otherFields() {
    return `
      <div class="row g-3">
        <div class="col-12">
          <label class="form-label">Other (describe your request)</label>
          <textarea class="form-control" name="other_description" rows="4" placeholder="Explain what you need (insurance, visa, etc.)..."></textarea>
        </div>
        <div class="col-md-6">
            <label class="form-label">Specify price range</label>
            <select class="form-select" name="other_price_range">
                <option value="">Select...</option>
                <option value="1">0 - 5.000€</option>
                <option value="2">5.000 - 15.000€</option>
                <option value="3">15.000 - 50.000€</option>
                <option value="4">More than 50.000€</option>
            </select>
        </div>
        <div class="col-md-6">
          <label class="form-label">Project</label>
          <select class="form-select" name="other_project">
            <option value="">Select...</option>
          </select>
        </div>

        <div class="col-12 d-none" data-procurement-warning="other">
          <div class="alert alert-warning mb-0" role="alert">
            Requests in this price range may require the initiation of a public procurement procedure. Management will contact you in the next few days.
          </div>
        </div>

        <div class="col-md-12 d-none" data-ip-wrapper="other">
          <label class="form-label">Send to responsible (PI, supervisor...)</label>
          <select class="form-select" name="other_ip">
            <option value="">Select responsible...</option>
          </select>
        </div>
      </div>

    `;
  }

  export function buildTravelIdentityBlock(scope = "main") {
    const prefix = scope === "edit" ? "edit-" : "";

    return `
      <div class="card border-warning-subtle bg-light mt-3" data-travel-identity-block="${scope}">
        <div class="card-body">
          <h6 class="mb-3">Travel contact and identification</h6>

          <div class="row g-4">
            <!-- PHONE BLOCK -->
                <div class="col-md-6">
                  <label class="form-label">Phone</label>
                  <input type="text" class="form-control" name="travel_contact_phone">
                </div>

                <div class="col-md-6" data-phone-confirm-wrapper="${scope}">
                  <label class="form-label d-block mb-2">Is this phone correct?</label>
                  <div>
                    <div class="form-check form-check-inline">
                      <input class="form-check-input" type="radio" name="travel_contact_phone_confirmed" value="yes" id="${prefix}travel-phone-yes">
                      <label class="form-check-label" for="${prefix}travel-phone-yes">Yes</label>
                    </div>
                    <div class="form-check form-check-inline">
                      <input class="form-check-input" type="radio" name="travel_contact_phone_confirmed" value="no" id="${prefix}travel-phone-no">
                      <label class="form-check-label" for="${prefix}travel-phone-no">No</label>
                    </div>
                  </div>
                </div>

                  <div class="col-12">
                    <div data-travel-phone-summary="${scope}"></div>
                  </div>
                </div>
              </div>
            </div>

            <!-- DOCUMENT BLOCK -->
            <div class="col-12">
              <div class="border rounded p-3 bg-white" id="stored-identity-box">
                <h6 class="mb-3 text-secondary">Identification document</h6>

                <div class="row g-3" id="travel-passport-fields">
                  <div class="col-md-4">
                    <label class="form-label">Document type</label>
                    <select class="form-select" name="travel_passport_doc_type">
                      <option value="">Select...</option>
                      <option value="passport">Passport</option>
                      <option value="dni">DNI</option>
                      <option value="nie">NIE</option>
                    </select>
                  </div>

                  <div class="col-md-4">
                    <label class="form-label">Document number</label>
                    <input type="text" class="form-control" name="travel_passport_doc_number">
                  </div>

                  <div class="col-md-4">
                    <label class="form-label">Expiration date</label>
                    <input type="date" class="form-control" name="travel_passport_expiration">
                  </div>

                  <div class="col-12">
                    <div data-travel-passport-summary="${scope}"></div>
                  </div>

                  <div class="col-12 d-none" data-travel-doc-expiration-warning="${scope}">
                    <div class="alert alert-danger mb-0" role="alert">
                      Your document will expire before the end of the trip. Please remember to renew it before travelling.
                    </div>
                  </div>

                  <div class="col-12" data-passport-confirm-wrapper="${scope}">
                    <label class="form-label d-block mb-2">Is this document correct?</label>
                    <div>
                      <div class="form-check form-check-inline">
                        <input class="form-check-input" type="radio" name="travel_passport_confirmed" value="yes" id="${prefix}travel-passport-yes">
                        <label class="form-check-label" for="${prefix}travel-passport-yes">Yes</label>
                      </div>
                      <div class="form-check form-check-inline">
                        <input class="form-check-input" type="radio" name="travel_passport_confirmed" value="no" id="${prefix}travel-passport-no">
                        <label class="form-check-label" for="${prefix}travel-passport-no">No</label>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    `;
  }

  export const SECTION_BUILDERS = {
    travel: (prefix = "main", parentId = "details-accordion") =>
      sectionTemplate("travel", "Travel", travelFields(), prefix, parentId),

    registration: (prefix = "main", parentId = "details-accordion") =>
      sectionTemplate("registration", "Registration", registrationFields(), prefix, parentId),

    accommodation: (prefix = "main", parentId = "details-accordion") =>
      sectionTemplate("accommodation", "Accommodation", accommodationFields(), prefix, parentId),

    equipment: (prefix = "main", parentId = "details-accordion") =>
      sectionTemplate("equipment", "Equipment", equipmentFields(), prefix, parentId),

    other: (prefix = "main", parentId = "details-accordion") =>
      sectionTemplate("other", "Other", otherFields(), prefix, parentId),
  };

  export const CATEGORY_ID_TO_KEY = {
    1: "travel",
    2: "registration",
    3: "accommodation",
    4: "equipment",
    5: "other"
  };

  export const STATUS_ORDER = {
    pending: 1,
    "approved-ip": 2,
    "approved-projects": 3,
    "approved-accounting": 4,
    "approved-it": 4,
    "rejected": 5,
    "rejected-ip": 5,
    "rejected-projects": 6,
    "rejected-it": 8,
    "rejected-accounting": 7,
    "canceled": 9
  };

  export function escapeHtml(str = "") {
    return String(str)
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;")
      .replaceAll("'", "&#039;");
  }

  export function formatDateTime(value) {
    if (!value) return "-";

    const str = String(value).trim();
    if (!str) return "-";

    const cleaned = str.replace("T", " ").replace("Z", "");
    return cleaned.slice(0, 16);
  }

  export function getStatusLabel(status) {
    const map = {
      pending: "Pending management",
      "approved-ip": "Approved by Responsible",
      "approved-projects": "Approved by Projects",
      "approved-accounting": "Approved by Accounting",
      "approved-it": "Approved by IT",
      "rejected-ip": "Rejected by Responsible",
      "rejected-projects": "Rejected by Projects",
      "rejected-it": "Rejected by IT",
      "rejected-accounting": "Rejected by Accounting",
      rejected: "Rejected"
    };

    return map[status] || status || "Unknown";
  }

  export function getStatusBadgeClass(status) {
    const map = {
      pending: "bg-warning text-dark ms-1",
      "approved-ip": "bg-info text-dark ms-1",
      "approved-projects": "bg-primary ms-1",
      "approved-accounting": "bg-success ms-1",
      "approved-it": "bg-success ms-1",
      "rejected-ip": "bg-danger ms-1",
      "rejected-projects": "bg-danger ms-1",
      "rejected-it": "bg-danger ms-1",
      "rejected-accounting": "bg-danger ms-1",
      rejected: "bg-danger ms-1"
    };

    return map[status] || "bg-secondary";
  }

  export function getCategoryLabel(categoryKey = "") {
    const map = {
      travel: "Travel",
      registration: "Registration",
      accommodation: "Accommodation",
      equipment: "Equipment",
      other: "Other"
    };

    return map[categoryKey] || categoryKey || "Unknown";
  }

  export function extractRequestData(raw) {
    if (!raw) return {};

    if (typeof raw.data === "object" && raw.data !== null) return raw.data;

    if (typeof raw.data === "string") {
      try {
        return JSON.parse(raw.data);
      } catch {
        return {};
      }
    }

    if (raw.combined_id || raw.parts || raw.creation_date) {
      return raw;
    }

    return {};
  }