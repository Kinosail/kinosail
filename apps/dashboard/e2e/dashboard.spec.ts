import { test } from "@playwright/test";
import { registerDashboardAccountTests } from "./dashboard-account-tests";
import { registerDashboardAuthTests } from "./dashboard-auth-tests";
import { registerDashboardBoardTests } from "./dashboard-board-tests";
import { registerDashboardResponsiveTests } from "./dashboard-responsive-tests";
import { registerDashboardSupporterTests } from "./dashboard-supporter-tests";

test.describe.serial("Kinosail Dashboard populated workflow", () => {
  registerDashboardAuthTests();
  registerDashboardBoardTests();
  registerDashboardAccountTests();
  registerDashboardResponsiveTests();
  registerDashboardSupporterTests();
});
