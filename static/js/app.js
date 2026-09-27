(() => {
	let timer;
	let lastActive;
	let subprojectRequest;

	const abortSubprojectRequest = () => {
		subprojectRequest?.abort();
		subprojectRequest = null;
	};

	const loadSubprojects = (select, projectID, emptyLabel) => {
		abortSubprojectRequest();
		select.value = "";
		const controller = new AbortController();
		subprojectRequest = controller;
		fetch(`/api/v1/subprojects?project_id=${encodeURIComponent(projectID)}`, {signal: controller.signal})
			.then((response) => response.ok ? response.json() : Promise.reject(response))
			.then((payload) => {
				if (subprojectRequest !== controller || !select.isConnected ||
					document.getElementById("task-subproject-filter") !== select ||
					document.getElementById("task-project-filter")?.value !== projectID) return;
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
				abortSubprojectRequest();
				subproject.replaceChildren(new Option("None", ""));
				return;
			}
			loadSubprojects(subproject, event.target.value, "None");
		} else if (event.target.id === "project-filter") {
			const form = event.target.closest("form");
			const subproject = form.querySelector("#subproject-filter");
			if (subproject) subproject.value = "";
			form.setAttribute("hx-params", "not subproject");
		} else if (event.target.id === "subproject-filter") {
			const form = event.target.closest("form");
			if (event.target.value) form.removeAttribute("hx-params");
			else form.setAttribute("hx-params", "not subproject");
		}
	}, true);

	document.addEventListener("focusin", (event) => {
		if (event.target.matches(".week-cell input")) lastActive = event.target;
	});

	const closeModal = () => {
		abortSubprojectRequest();
		document.getElementById("modal-root")?.replaceChildren();
	};

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

	document.body.addEventListener("htmx:beforeSwap", (event) => {
		if (event.detail.target?.id === "modal-root") abortSubprojectRequest();
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
