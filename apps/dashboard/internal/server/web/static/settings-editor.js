import { api, post, put, remove } from "./api.js";
import { button, clear, element, field, selectChoice, selectedChoice } from "./dom.js";

export function createSettingsEditor(state, refresh, toast) {
	const settingsDialog = field("settings-dialog");
	let externalImport = null;

	settingsDialog.addEventListener("close", () => {
		field("mcp-token-value").value = "";
		field("mcp-token-output").hidden = true;
		clearPasswords();
		const url = new URL(location.href);
		if (url.searchParams.get("passkey") === "offer") {
			url.searchParams.delete("passkey");
			history.replaceState(null, "", url.pathname + url.search + url.hash);
		}
	});

	function clearPasswords() {
		field("current-password").value = "";
		field("new-password").value = "";
		field("confirm-new-password").value = "";
	}

	function clearExternalImport() {
		externalImport = null;
		field("external-import-preview").hidden = true;
		field("external-import-file").value = "";
	}

	function renderExternalPreview(preview) {
		field("external-import-title").textContent = `Applications ready to import from ${preview.source}: ${preview.apps.length}`;
		field("external-import-summary").textContent = "These applications will be added to the current board. Existing addresses are skipped. Review the direct addresses before confirming.";
		clear(field("external-import-list"));
		preview.apps.slice(0, 80).forEach((app) => field("external-import-list").append(element("li", "", `${app.name} · ${app.url}`)));
		field("external-import-warning").textContent = (preview.warnings || []).join(" ");
		field("external-import-preview").hidden = false;
	}

	function openSettings(options = {}) {
		const passkeyOffer = options.passkeyOffer === true;
		settingsDialog.classList.toggle("passkey-offer", passkeyOffer);
		field("settings-dialog-title").textContent = passkeyOffer ? "Make the next sign-in easier" : "Settings and backup";
		field("passkey-not-now").hidden = !passkeyOffer;
		field("settings-title").value = state.board.title;
		field("settings-error").hidden = true;
		clearPasswords();
		field("mcp-token-value").value = "";
		field("mcp-token-output").hidden = true;
		renderAudit();
		renderRemoved();
		clearExternalImport();
		settingsDialog.showModal();
		if (passkeyOffer) field("add-passkey-button").focus();
	}

	function renderAudit() {
		clear(field("audit-list"));
		(state.board.audit || []).slice(0, 12).forEach((item) => {
			const row = element("li");
			const copy = element("span");
			copy.append(element("strong", "", item.subject), document.createTextNode(` · ${item.action.replace(".", " ")}`));
			row.append(copy, element("time", "", new Date(item.at).toLocaleString()));
			field("audit-list").append(row);
		});
	}

	function renderRemoved() {
		clear(field("removed-list"));
		const recentlyRemoved = state.board.recentlyRemoved || [];
		if (!recentlyRemoved.length) field("removed-list").append(element("p", "", "No recently removed applications."));
		recentlyRemoved.forEach((item) => {
			const row = element("div", "removed-row");
			row.append(element("span", "", item.app.name));
			const restore = button("text-button", "Restore");
			restore.addEventListener("click", async () => {
				try {
					await post(`/api/v1/apps/${item.app.id}/restore`, {
						expectedVersion: state.board.version,
					});
					settingsDialog.close();
					await refresh();
					toast(`${item.app.name} restored`);
				} catch (error) {
					showSettingsError(error.message);
				}
			});
			row.append(restore);
			field("removed-list").append(row);
		});
	}

	function showSettingsError(message) {
		field("settings-error").textContent = message;
		field("settings-error").hidden = false;
	}

	field("external-import-source").addEventListener("change", clearExternalImport);
	field("settings-form").addEventListener("submit", async (event) => {
		event.preventDefault();
		const settingsForm = event.currentTarget;
		if (settingsForm.getAttribute("aria-busy") === "true") return;
		const save = settingsForm.querySelector('[type="submit"]');
		settingsForm.setAttribute("aria-busy", "true");
		save.disabled = true;
		save.textContent = "Saving board…";
		field("settings-error").hidden = true;
		try {
			const result = await put("/api/v1/board", {
				title: field("settings-title").value,
				expectedVersion: state.board.version,
			});
			settingsDialog.close();
			await refresh();
			toast(result.receipt.message);
		} catch (error) {
			showSettingsError(error.message);
		} finally {
			save.disabled = false;
			save.textContent = "Save board";
			settingsForm.removeAttribute("aria-busy");
		}
	});

	field("import-file").addEventListener("change", async (event) => {
		const file = event.target.files[0];
		if (!file || file.size > 2 * 1024 * 1024) {
			showSettingsError("Choose a JSON backup no larger than 2 MiB.");
			return;
		}
		try {
			const backup = JSON.parse(await file.text());
			backup.expectedVersion = state.board.version;
			const result = await put("/api/v1/import", backup);
			settingsDialog.close();
			await refresh();
			toast(result.receipt.message);
		} catch (error) {
			showSettingsError(error.message || "Could not read this backup. Choose a valid Dashboard JSON backup.");
		}
	});

	field("external-import-file").addEventListener("change", async (event) => {
		const file = event.target.files[0];
		field("settings-error").hidden = true;
		if (!file || file.size > 2 * 1024 * 1024) {
			showSettingsError("Choose a dashboard configuration file no larger than 2 MiB.");
			return;
		}
		try {
			const content = await file.text();
			const source = detectImportSource(content, file.name);
			selectChoice("external-import-source", source);
			const preview = await post("/api/v1/import/preview", { source, content });
			externalImport = { source, content };
			renderExternalPreview(preview);
		} catch (error) {
			showSettingsError(error.message || "Could not read this configuration file. Check the selected import format.");
		}
	});

	function detectImportSource(content, filename) {
		const lowerName = filename.toLocaleLowerCase();
		if (lowerName.includes("homepage")) return "homepage";
		if (lowerName.includes("homarr")) return "homarr";
		if (lowerName.includes("dashy") || /(?:^|\n)\s*(?:pageInfo|sections):/u.test(content)) return "dashy";
		if (/(?:^|\n)\s*(?:services|bookmarks):/u.test(content)) return "homepage";
		return selectedChoice("external-import-source");
	}

	field("external-import-confirm").addEventListener("click", async () => {
		if (!externalImport) return;
		const control = field("external-import-confirm");
		control.disabled = true;
		try {
			const result = await post("/api/v1/import/external", {
				...externalImport,
				expectedVersion: state.board.version,
			});
			settingsDialog.close();
			await refresh();
			toast(result.receipt.message);
		} catch (error) {
			showSettingsError(error.message || "Import could not be completed.");
		} finally {
			control.disabled = false;
		}
	});

	field("create-mcp-token").addEventListener("click", async () => {
		try {
			const result = await post("/api/v1/mcp-token", {});
			field("mcp-token-value").value = result.token;
			field("mcp-token-output").hidden = false;
			field("mcp-token-value").focus();
			field("mcp-token-value").select();
			toast("MCP token created. Save it now.");
		} catch (error) {
			showSettingsError(error.message);
		}
	});

	field("copy-mcp-token").addEventListener("click", async () => {
		try {
			await navigator.clipboard.writeText(field("mcp-token-value").value);
			toast("MCP token copied");
		} catch {
			field("mcp-token-value").focus();
			field("mcp-token-value").select();
			toast("Copy the selected token");
		}
	});

	field("revoke-mcp-token").addEventListener("click", async () => {
		try {
			await remove("/api/v1/mcp-token", {});
			field("mcp-token-value").value = "";
			field("mcp-token-output").hidden = true;
			toast("MCP token revoked");
		} catch (error) {
			showSettingsError(error.message);
		}
	});

	field("change-password-button").addEventListener("click", async () => {
		const control = field("change-password-button");
		const currentPassword = field("current-password").value;
		const newPassword = field("new-password").value;
		const confirmPassword = field("confirm-new-password").value;
		field("settings-error").hidden = true;
		if (newPassword !== confirmPassword) {
			showSettingsError("New passwords do not match.");
			field("confirm-new-password").focus();
			return;
		}
		control.disabled = true;
		try {
			await post("/api/v1/owner/password", {
				currentPassword,
				newPassword,
				confirmPassword,
			});
			clearPasswords();
			toast("Password changed. Other sessions were signed out.");
		} catch (error) {
			showSettingsError(error.message);
		} finally {
			control.disabled = false;
		}
	});

	field("sign-out-button").addEventListener("click", async () => {
		await api("/api/v1/session", { method: "DELETE" });
		location.assign("/login");
	});
	return openSettings;
}
