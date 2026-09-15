import { get, post, setCSRF } from "./api.js";

const livingNames = ["Signal Glow", "Dock Pulse", "Route Current", "Port Watch", "Service Beacon", "Harbor Standard", "Operator Array", "Fleet Signal", "Command Halo", "Eternal Uplink"];
const patronNames = ["First Pin", "Dock Marker", "Route Compass", "Port Shield", "Service Medal", "Harbor Crest", "Operator Order", "Fleet Star", "Command Crown", "Sovereign Console"];
const masterworkNames = ["Joined Signal", "Twin Port", "Route Accord", "Harbor Union", "Service Command", "Operator Bridge", "Fleet Vanguard", "Admiral Console", "Crowned Fleet", "Sovereign Fleet Command"];

const field = id => document.getElementById(id);

function badge(name, rank, collected, family) {
  const item = document.createElement("li");
  item.className = `case-badge rank-${rank}${collected ? " is-collected" : ""}${rank >= 7 ? " is-elite" : ""}`;
  const mark = document.createElement("span");
  mark.className = `badge-mark ${family}`;
  mark.setAttribute("aria-hidden", "true");
  mark.innerHTML = "<i></i><b></b><em></em>";
  const copy = document.createElement("span");
  const level = document.createElement("small");
  level.textContent = `${rank} / 10${rank >= 7 ? " · Command class" : ""}`;
  const title = document.createElement("strong");
  title.textContent = name;
  copy.append(level, title);
  item.append(mark, copy);
  return item;
}

function renderFamily(id, names, level, family) {
  const list = field(id);
  list.replaceChildren(...names.map((name, index) => badge(name, index + 1, index < level, family)));
}

function render(status) {
  const badgeCase = status.badgeCase;
  field("unlocked-count").textContent = badgeCase.unlocked;
  renderFamily("living-badges", livingNames, badgeCase.livingLevel, "living");
  renderFamily("patron-badges", patronNames, badgeCase.patronLevel, "patron");
  field("living-state").textContent = status.livingStandard ? (status.livingStandard.active ? `Level ${badgeCase.livingLevel} · Signal active` : `Level ${badgeCase.livingLevel} · Archived`) : "Not collected";
  field("patron-state").textContent = status.patronOrder ? `Level ${badgeCase.patronLevel} · Permanent` : "Not collected";

  const masterwork = field("masterwork");
  masterwork.className = `masterwork ${badgeCase.masterworkEarned ? (badgeCase.masterworkActive ? "is-active" : "is-dormant") : "is-empty"}`;
  field("masterwork-kicker").textContent = badgeCase.masterworkEarned ? `Dashboard masterwork · Level ${badgeCase.masterworkLevel}` : "Dashboard masterwork";
  field("masterwork-title").textContent = badgeCase.masterworkEarned ? masterworkNames[badgeCase.masterworkLevel - 1] : "Fleet Command awaits";
  field("masterwork-copy").textContent = badgeCase.masterworkEarned ? "Fleet Command combines both Dashboard badge families. Its level increases when both families reach the next level." : "Collect a badge from each family to earn Fleet Command. Its level matches the lower of the two.";
  field("masterwork-state").textContent = badgeCase.masterworkEarned ? (badgeCase.masterworkActive ? "Fleet signal active" : "Earned permanently · Signal dormant") : "Not yet forged";
  field("activation-form").hidden = !status.activationAvailable;
  if (!status.activationAvailable) field("activate-title").textContent = "Activation is not configured";
  field("supporter-loading").hidden = true;
  field("supporter-content").hidden = false;
}

async function start() {
  try {
    const [me, status] = await Promise.all([get("/api/v1/me"), get("/api/v1/supporter")]);
    setCSRF(me.csrf);
    render(status);
  } catch (error) {
    field("supporter-loading").textContent = error.message;
  }
}

field("activation-form").addEventListener("submit", async event => {
  event.preventDefault();
  const button = event.currentTarget.querySelector("button");
  const message = field("activation-message");
  button.disabled = true;
  message.textContent = "Verifying your supporter badge…";
  try {
    const name = field("recognition-name").value;
    const input = { key: field("supporter-key").value };
    if (name) input.recognitionName = name;
    const status = await post("/api/v1/supporter/activate", input);
    field("supporter-key").value = "";
    message.textContent = "Supporter badge verified and saved on this Dashboard.";
    render(status);
  } catch (error) {
    message.textContent = error.message;
  } finally {
    button.disabled = false;
  }
});

start();
