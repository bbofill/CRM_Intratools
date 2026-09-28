import { measurePageLoadTime } from "/assets/js/checkLoadTime.js?v={{.AppVersion}}";
import { initNotificationBadge } from "/assets/js/notificationUtils.js?v={{.AppVersion}}";
import "/assets/js/tutorials/tutorial_module10.js";
import { initTravelMode } from "./travelMode.js";
import { initExpenses } from "./expenses.js";
import { initLatestStatusModal } from "./latestStatus.js";
import { initCommissionForm, lockFutureTravelDates } from "./commissionForm.js";
import { initCommissionsWorkspace } from "./commissions.js";
import { initRatesManagement } from "./ratesManagement.js";

document.addEventListener("DOMContentLoaded", async () => {
  measurePageLoadTime({
    targetId: "load-info",
    precision: 6,
    onComplete: (time) => console.log("Load Time:", `${time}s`)
  });

  initNotificationBadge();

  await initTravelMode();
  initExpenses();
  initLatestStatusModal();
  initCommissionForm();
  lockFutureTravelDates();
  await initCommissionsWorkspace();
  await initRatesManagement();
});
