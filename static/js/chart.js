(() => {
	if (!window.Chart) return;
	const Chart = window.Chart;
	const dark = window.matchMedia("(prefers-color-scheme: dark)");

	// Approximates views.hours; the two round exact .x5 boundaries differently.
	const formatHours = (v) => String(Number((Math.round(v * 10) / 10).toFixed(1)));

	const applyTheme = () => {
		Chart.defaults.color = dark.matches ? "#d6d6d6" : "#333";
		Chart.defaults.borderColor = dark.matches ? "#3a3a3a" : "#ddd";
		for (const chart of Object.values(Chart.instances)) chart.update();
	};

	const render = () => {
		for (const holder of document.querySelectorAll("[data-chart]")) {
			const canvas = holder.querySelector("canvas");
			if (!canvas || !holder.dataset.series || Chart.getChart(canvas)) continue;
			const styles = getComputedStyle(document.documentElement);
			const colors = ["--pico-primary", "--pico-del-color", "--pico-ins-color"].map((name) => styles.getPropertyValue(name).trim());
			let data;
			try {
				data = JSON.parse(holder.dataset.series);
			} catch {
				continue;
			}
			data.datasets.forEach((dataset, index) => {
				dataset.borderColor = colors[index] || colors[0];
				dataset.backgroundColor = dataset.borderColor;
				dataset.tension = 0.2;
				dataset.pointRadius = 3;
			});
			new Chart(canvas, {
				type: "line",
				data,
				options: {
					responsive: true,
					maintainAspectRatio: false,
					scales: { y: { beginAtZero: true, title: { display: true, text: "Hours [h]" } } },
					plugins: { tooltip: { callbacks: { label: (c) => `${c.dataset.label.replace(/\s\[h\]$/, "")}: ${formatHours(c.parsed.y)} h` } } },
				},
			});
		}
	};

	applyTheme();
	dark.addEventListener("change", applyTheme);
	render();
	document.body.addEventListener("htmx:after:swap", render);
})();
