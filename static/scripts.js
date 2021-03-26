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
	}, 5*60*1000);
}
