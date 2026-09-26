(() => {
	const dark = window.matchMedia("(prefers-color-scheme: dark)");

	const applyTheme = () => {
		Chart.defaults.color = dark.matches ? "#d6d6d6" : "#333";
		Chart.defaults.borderColor = dark.matches ? "#3a3a3a" : "#ddd";
		for (const chart of Object.values(Chart.instances)) chart.update();
	};

	const render = () => {
		for (const holder of document.querySelectorAll("[data-chart]")) {
			const canvas = holder.querySelector("canvas");
			if (!canvas || !holder.dataset.series || Chart.getChart(canvas)) continue;
			new Chart(canvas, {
				type: "line",
				data: JSON.parse(holder.dataset.series),
				options: { scales: { y: { beginAtZero: true } } },
			});
		}
	};

	applyTheme();
	dark.addEventListener("change", applyTheme);
	render();
	document.body.addEventListener("htmx:afterSwap", render);
})();
