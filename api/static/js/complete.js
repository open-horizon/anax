/* The JavaScript that is used in complete.html for device registration */
var ref = 'http://' + location.hostname + ':8000/';
document.getElementById('exchangeDisplay').innerHTML = ref;

var check = function() {
    $.ajax({
        url: ref + 'mtn/marketplace/v1/user',
        type: 'GET',
        contentType: 'application/json',
        datatType: 'json',
    }).done(function(data) {
        console.log("Data received from exchange", data);
        window.location.href = ref;
    }).fail(function() {
        setTimeout(check, 5000);
    });
};

setTimeout(check, 4000);
