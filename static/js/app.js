(() => {
	let timer;

	document.body.addEventListener("htmx:responseError", (event) => {
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
