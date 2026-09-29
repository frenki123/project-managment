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
			const subproject = event.target.closest("form")?.querySelector("#subproject-filter");
			if (subproject) subproject.value = "";
		}
	}, true);

	document.addEventListener("focusin", (event) => {
		if (event.target.matches(".week-cell input")) lastActive = event.target;
	});

	const closeModal = () => {
		abortSubprojectRequest();
		const modal = document.getElementById("modal-root");
		const backdrop = document.getElementById("modal-backdrop");
		modal?.replaceChildren();
		modal?.classList.remove("is-form");
		modal?.setAttribute("aria-hidden", "true");
		if (backdrop) backdrop.hidden = true;
		document.body.classList.remove("modal-open");
		if (modalOpener?.isConnected) modalOpener.focus();
		modalOpener = null;
	};

	const focusableSelector = "button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex=\"-1\"])";

	const showModal = () => {
		const modal = document.getElementById("modal-root");
		if (!modal || !modal.hasChildNodes()) return;
		const backdrop = document.getElementById("modal-backdrop");
		modal.classList.toggle("is-form", Boolean(modal.querySelector(".resource-form")));
		modal.setAttribute("aria-hidden", "false");
		if (backdrop) backdrop.hidden = false;
		document.body.classList.add("modal-open");
		const first = modal.querySelector("[autofocus], .panel-close, input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled])");
		(first || modal).focus();
	};

	const setupDateRange = () => {
		const start = document.getElementById("project-start-date");
		const end = document.getElementById("project-end-date");
		const message = document.getElementById("project-date-error");
		if (!start || !end) return;
		if (start.value) end.min = start.value;
		else end.removeAttribute("min");
		if (end.value) start.max = end.value;
		else start.removeAttribute("max");
		const invalid = Boolean(start.value && end.value && end.value < start.value);
		end.setCustomValidity(invalid ? "End date must be on or after start date." : "");
		if (message) message.hidden = !invalid;
	};

	document.addEventListener("click", (event) => {
		const trigger = event.target.closest('[hx-target="#modal-root"]');
		if (trigger && !document.getElementById("modal-root")?.contains(trigger)) {
			modalOpener = trigger;
		}
		if (event.target.closest("[data-close-modal]")) closeModal();
		if (event.target.id === "modal-backdrop") closeModal();
	});

	document.addEventListener("keydown", (event) => {
		const modal = document.getElementById("modal-root");
		if (event.key === "Escape" && modal?.hasChildNodes()) {
			closeModal();
			return;
		}
		if (event.key !== "Tab" || !modal?.hasChildNodes()) return;
		const focusable = [...modal.querySelectorAll(focusableSelector)];
		if (!focusable.length) {
			event.preventDefault();
			modal.focus();
			return;
		}
		const first = focusable[0];
		const last = focusable[focusable.length - 1];
		if (event.shiftKey && document.activeElement === first) {
			event.preventDefault();
			last.focus();
		} else if (!event.shiftKey && document.activeElement === last) {
			event.preventDefault();
			first.focus();
		}
	});

	document.addEventListener("input", (event) => {
		if (event.target.matches("#project-start-date, #project-end-date")) setupDateRange();
	});

	document.addEventListener("change", (event) => {
		if (event.target.matches("#project-start-date, #project-end-date")) setupDateRange();
	});

	const showToast = (message) => {
		const toast = document.getElementById("toast");
		if (!toast) return;
		toast.textContent = message;
		toast.hidden = false;
		clearTimeout(timer);
		timer = setTimeout(() => {
			toast.hidden = true;
		}, 5000);
	};

	document.body.addEventListener("app:toast", (event) => {
		showToast(event.detail?.message || "Request could not be completed");
	});

	document.body.addEventListener("htmx:after:swap", (event) => {
		const target = event.detail.target || event.detail.ctx?.target;
		if (target?.id === "modal-root") {
			showModal();
			setupDateRange();
		}
		if (!lastActive) return;
		if (!target?.matches("tr.task-row")) return;
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
	});

	document.addEventListener("DOMContentLoaded", () => {
		setupDateRange();
	});

})();
