(() => {
	let timer;
	let lastActive;
	let subprojectRequest;
	let modalOpener;

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
			.catch((error) => {
				if (error.name === "AbortError" || subprojectRequest !== controller || !select.isConnected) return;
				select.replaceChildren(new Option("Unable to load subprojects", ""));
			});
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
		if (modalOpener?.isConnected) modalOpener.focus();
		modalOpener = null;
	};

	document.addEventListener("click", (event) => {
		const trigger = event.target.closest('[hx-target="#modal-root"]');
		if (trigger && !document.getElementById("modal-root")?.contains(trigger)) modalOpener = trigger;
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

	document.body.addEventListener("htmx:after:swap", (event) => {
		if (!lastActive) return;
		if (!event.detail.ctx.target?.matches("tr.task-row")) return;
		const selector = `[name="${lastActive.name}"][data-save-path="${lastActive.dataset.savePath || ""}"]`;
		const replacement = document.querySelector(selector);
		if (replacement) replacement.focus();
		lastActive = null;
	});

	document.body.addEventListener("htmx:before:swap", (event) => {
		if (event.detail.ctx.target?.id === "modal-root") abortSubprojectRequest();
	});

	document.body.addEventListener("htmx:before:request", (event) => {
		const trigger = event.detail.ctx.sourceElement;
		if (trigger?.getAttribute("hx-target") === "#modal-root" && !document.getElementById("modal-root")?.contains(trigger)) {
			modalOpener = trigger;
		}
	});

	document.body.addEventListener("htmx:response:error", (event) => {
		const ctx = event.detail.ctx;
		if (ctx.sourceElement?.matches(".week-cell input")) lastActive = null;
		const toast = document.getElementById("toast");
		if (!toast) {
			return;
		}
		toast.textContent = ctx.text || `Request failed (${ctx.response?.status})`;
		toast.hidden = false;
		clearTimeout(timer);
		timer = setTimeout(() => {
			toast.hidden = true;
		}, 5000);
	});
})();
