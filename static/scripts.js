function reloadCaptcha() {
	var e = document.getElementById("captcha-image");
	var q = "reload=" + (new Date()).getTime();
	var src  = e.src;
	var p = src.indexOf('?');
	if (p >= 0) {
		src = src.substr(0, p);
	}
	e.src = src + "?" + q
}

function scheduleReload() {
	setTimeout(function(){
		window.location.reload(1);
	}, 5*60*1000); // 5 minutes
}

function updateHealth(url) {
	var xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if(xhr.readyState == 4) {
			if(xhr.status == 200) {
				let data = JSON.parse(xhr.responseText);
				let text = "";
				for(d of data) {
					text = text + `<span class="badge bg-${d.Synced ? 'success' : 'danger'}">${d.CryptoCode}: ${d.Synced ? 'synced' : 'out of sync'}</span> `;
				}
				document.getElementById("health-widget").innerHTML = text;
			} else {
				document.getElementById("health-widget").innerHTML = '<span class="badge bg-warning">could not connect</span>';
			}
		}
	}
	xhr.open("GET", url, true); // true for asynchronous
	xhr.send(null);

	setTimeout(function(){
		updateHealth(url);
	}, 10*1000); // 10 seconds
}
