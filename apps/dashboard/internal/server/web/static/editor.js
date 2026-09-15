import { patch, post, put, remove } from "./api.js";
import { brandMark, button, clear, element, field, selectChoice, selectedChoice } from "./dom.js";
import { createSettingsEditor } from "./settings-editor.js";

export function createEditor(state, refresh, toast) {
	const appDialog = field("app-dialog");
	const form = field("app-form");
	let selectedCatalog = "";
	let catalogQuery = "";
	let removeArmed = false;
	const openSettings = createSettingsEditor(state, refresh, toast);

	document.querySelectorAll("[data-close-dialog]").forEach((control) => control.addEventListener("click", () => control.closest("dialog").close()));
	field("catalog-search").addEventListener("input", (event) => {
		catalogQuery = event.target.value;
		renderCatalog();
	});
	appDialog.addEventListener("close", () => {
		removeArmed = false;
		field("remove-button").dataset.confirm = "false";
	});

	function renderCatalog() {
		const picker = field("catalog-picker");
		clear(picker);
		const query = catalogQuery.trim().toLocaleLowerCase();
		const entries = state.catalog.filter((entry) => !query || `${entry.name} ${entry.category} ${entry.description}`.toLocaleLowerCase().includes(query));
		entries.forEach((entry) => {
			const option = button("catalog-option", "", "");
			option.dataset.id = entry.id;
			option.setAttribute("aria-pressed", String(selectedCatalog === entry.id));
			option.setAttribute("aria-label", entry.name);
			option.append(brandMark(entry.name, entry.id, entry.accent), element("span", "", entry.name), element("small", "", entry.category));
			option.addEventListener("click", () => selectCatalog(entry));
			picker.append(option);
		});
		if (!entries.length) picker.append(element("p", "catalog-empty", "No known applications match that search."));
	}

	function selectCatalog(entry) {
		selectedCatalog = entry.id;
		field("app-name").value = entry.name;
		field("app-category").value = entry.category;
		field("app-description").value = entry.description;
		selectChoice("app-accent", entry.accent);
		renderCatalog();
		field("app-url").focus();
	}

	function resetForm() {
		form.reset();
		selectedCatalog = "";
		catalogQuery = "";
		field("catalog-search").value = "";
		field("app-id").value = "";
		selectChoice("app-accent", "slate");
		field("app-check-enabled").checked = true;
		field("app-dialog-title").textContent = "Add application";
		field("save-app-button").textContent = "Add application";
		field("remove-button").hidden = true;
		field("remove-button").textContent = "Remove from Dashboard";
		field("app-form-error").hidden = true;
		renderCatalog();
	}

	function openAdd() {
		resetForm();
		appDialog.showModal();
		field("app-name").focus();
	}

	function openEdit(app) {
		resetForm();
		field("app-id").value = app.id;
		field("app-name").value = app.name;
		field("app-url").value = app.url;
		field("app-health-url").value = app.healthUrl === app.url ? "" : app.healthUrl;
		field("app-description").value = app.description || "";
		field("app-category").value = app.category || "";
		selectChoice("app-accent", app.accent);
		field("app-check-enabled").checked = app.checkEnabled;
		field("app-favorite").checked = app.favorite;
		field("app-dialog-title").textContent = `Edit ${app.name}`;
		field("save-app-button").textContent = "Save changes";
		field("remove-button").hidden = false;
		appDialog.showModal();
		field("app-name").focus();
	}

	form.addEventListener("submit", async (event) => {
		event.preventDefault();
		if (form.getAttribute("aria-busy") === "true") return;
		const id = field("app-id").value;
		const payload = {
			name: field("app-name").value,
			url: field("app-url").value,
			healthUrl: field("app-health-url").value,
			description: field("app-description").value,
			category: field("app-category").value,
			accent: selectedChoice("app-accent"),
			checkEnabled: field("app-check-enabled").checked,
			icon: selectedCatalog ? state.catalog.find((item) => item.id === selectedCatalog)?.icon : id ? state.board.apps.find((item) => item.id === id)?.icon : "app",
			favorite: field("app-favorite").checked,
			expectedVersion: state.board.version,
		};
		if (!id && selectedCatalog) payload.catalogId = selectedCatalog;
		const save = field("save-app-button");
		const label = save.textContent;
		save.disabled = true;
		save.textContent = id ? "Saving changes…" : "Adding application…";
		form.setAttribute("aria-busy", "true");
		field("app-form-error").hidden = true;
		try {
			const result = id ? await patch(`/api/v1/apps/${id}`, payload) : await post("/api/v1/apps", payload);
			appDialog.close();
			toast(result.receipt.message);
			await refresh();
		} catch (error) {
			field("app-form-error").textContent = error.message;
			field("app-form-error").hidden = false;
		} finally {
			save.disabled = false;
			save.textContent = label;
			form.removeAttribute("aria-busy");
		}
	});

	field("remove-button").addEventListener("click", async () => {
		if (!removeArmed) {
			removeArmed = true;
			field("remove-button").dataset.confirm = "true";
			field("remove-button").textContent = "Confirm removal";
			return;
		}
		const id = field("app-id").value;
		try {
			const result = await remove(`/api/v1/apps/${id}`, {
				expectedVersion: state.board.version,
			});
			appDialog.close();
			await refresh();
			toast(result.receipt.message, "Undo", async () => {
				await post(`/api/v1/apps/${id}/restore`, {
					expectedVersion: state.board.version,
				});
				await refresh();
				toast("Application restored");
			});
		} catch (error) {
			field("app-form-error").textContent = error.message;
			field("app-form-error").hidden = false;
		}
	});

	async function move(id, delta) {
		const ids = state.board.apps.map((app) => app.id);
		const index = ids.indexOf(id);
		const target = index + delta;
		if (target < 0 || target >= ids.length) return;
		[ids[index], ids[target]] = [ids[target], ids[index]];
		const movedName = state.board.apps[index].name;
		await saveOrder(ids, state.board.version);
		toast(`${movedName} moved ${delta < 0 ? "earlier" : "later"}`);
	}

	async function saveOrder(ids, version) {
		let result;
		try {
			result = await put("/api/v1/apps/order", {
				ids,
				expectedVersion: version,
			});
		} catch (error) {
			try {
				await refresh();
			} catch {
				/* Keep the original reorder error. */
			}
			throw error;
		}
		try {
			await refresh();
		} catch {
			const error = new Error("Board order was saved. Refresh to load it.");
			error.refreshOnly = true;
			throw error;
		}
		return result;
	}

	return { openAdd, openEdit, openSettings, move, saveOrder };
}
