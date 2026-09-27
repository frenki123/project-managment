(() => {
	let timer;
	let lastActive;
	let subprojectRequest;

	const loadSubprojects = (select, projectID, emptyLabel) => {
		if (subprojectRequest) subprojectRequest.abort();
		select.value = "";
		const controller = new AbortController();
		subprojectRequest = controller;
		fetch(`/api/v1/subprojects?project_id=${encodeURIComponent(projectID)}`, {signal: controller.signal})
			.then((response) => response.ok ? response.json() : Promise.reject(response))
			.then((payload) => {
				select.replaceChildren(new Option(emptyLabel, ""));
				for (const item of payload.subprojects || []) select.append(new Option(item.name, item.id));
			})
			.catch(() => {});
	};

	document.addEventListener("change", (event) => {
		if (event.target.id === "task-project-filter") {
			const subproject = document.getElementById("task-subproject-filter");
			if (!subproject) return;
			if (!event.target.value) {
				subproject.replaceChildren(new Option("None", ""));
				return;
			}
			loadSubprojects(subproject, event.target.value, "None");
		}
	});

	document.addEventListener("focusin", (event) => {
		if (event.target.matches(".week-cell input")) lastActive = event.target;
	});

	const closeModal = () => document.getElementById("modal-root")?.replaceChildren();

	document.addEventListener("click", (event) => {
		if (event.target.closest("[data-close-modal]")) closeModal();
	});

	document.addEventListener("keydown", (event) => {
		if (event.key === "Escape") {
			closeModal();
		}
	});

	document.addEventListener("submit", (event) => {
		const message = event.target.dataset.confirm;
		if (message && !confirm(message)) event.preventDefault();
	});

	document.body.addEventListener("htmx:afterSwap", (event) => {
		if (!lastActive) return;
		if (!event.detail.target?.matches("tr.task-row")) return;
		const selector = `[name="${lastActive.name}"][data-save-path="${lastActive.dataset.savePath || ""}"]`;
		const replacement = document.querySelector(selector);
		if (replacement) replacement.focus();
		lastActive = null;
	});

	document.body.addEventListener("htmx:responseError", (event) => {
		if (event.detail.elt?.matches(".week-cell input")) lastActive = null;
		const toast = document.getElementById("toast");
		if (!toast) {
			return;
		}
		const response = event.detail.xhr;
		toast.textContent = response.responseText || `Request failed (${response.status})`;
		toast.hidden = false;
		clearTimeout(timer);
		timer = setTimeout(() => {
			toast.hidden = true;
		}, 5000);
	});
})();
