import {test} from "@playwright/test";
import {configureTestInstance} from "./test-instance-helpers";
import {runLoadingIntent} from "./queue-loading-intent-helpers";

configureTestInstance();
for (const command of ["seek", "stop"] as const) {
  test(`real queued track preserves ${command} arriving before actual metadata`, {tag: ["@smoke", "@network-delay"]},
    ({page}, info) => runLoadingIntent(page, info, command));
  test(`real queued saved35 track preserves ${command} arriving before actual metadata`, {tag: ["@smoke", "@network-delay"]},
    ({page}, info) => runLoadingIntent(page, info, command, "Queue Intent Long Session", 35));
}
