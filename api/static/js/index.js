/* The JavaScript that is used in index.html for device registration */
// try to fetch starting geo facts if this is the first time this page was brought up
function getGeo() {
    var saved_lat = sessionStorage.getItem('lat');
    var saved_lon = sessionStorage.getItem('lon');
    var cur_lat = document.getElementById('lat').value;
    var cur_lon = document.getElementById('lon').value;
    if (!saved_lat || saved_lat === '' ||
        !saved_lon || saved_lon === '' ||
        !cur_lat || cur_lat === '' ||
        !cur_lon || cur_lon === '') {
        var retry = 0;

        function doQuery() {
            $.ajax({
                url: 'http://ip-api.com/json',
                type: 'GET',
                crossDomain: true,
                dataType: 'json',
                success: function(data) {
                    if (data.hasOwnProperty('lat')) {
                        document.getElementById('lat').value = data.lat;
                        document.getElementById('lon').value = data.lon;
                    } else {
                        console.log("Could not get geo from ip-api.com.");
                        retry++;
                        if (retry < 5) {
                            setTimeout(doQuery, 2000);
                        }
                    }
                },
                error: function(xhr, status, err) {
                    console.log("Error getting geo from ip-api.com: " + status + err);
                    retry++;
                    if (retry < 5) {
                        setTimeout(doQuery, 2000);
                    }
                },
            });
        }
        doQuery();
    }
}

function saveGeo() {
    sessionStorage.setItem("lat", $('#lat').val());
    sessionStorage.setItem("lon", $('#lon').val());
}

// redirect if iotf contract is true
if (config.show_applications.iotf) {
    window.location = '/registration/iotf.html';
}

// redirect if contracts already in db
$.ajax({
    url: '/contract',
    type: 'GET',
    crossDomain: true,
    dataType: 'json',
    success: function(data) {
        if (data.length > 0) {
            window.location = "/registration/status.html";
        }
    },
});

// check internet connectivity
$.ajax({
    url: '/info',
    type: 'GET',
    crossDomain: true,
    dataType: 'json',
    success: function(data) {
        var bad_servers = [];
        for (var key in data.connectivity) {
            if (!data.connectivity[key]) {
                bad_servers.push(key);
            }
        }

        if (bad_servers.length == 0) {
            disp_app_section(0, 'precheck');
        } else {
            document.getElementById('precheck_warn').innerHTML = "The ip address for the following servers cannot be resolved:\n&nbsp;&nbsp;&nbsp;&nbsp;" + bad_servers.join();
            disp_app_section(1, 'precheck');
        }
        var str = JSON.stringify(data.connectivity, null, 2);
        console.log("Checking internet connectivity: " + str);
    },
    error: function(xhr, status, err) {
        document.getElementById('precheck_warn').innerHTML = ""
        disp_app_section(1, 'precheck');
        console.log("Error checking internet connectivity: " + status + err);
    },
});

disp_app_section($('#devmode').prop("checked"), 'start_local_gov_section');


// show the "Start Local AgreementBot" checkbox when devmode checkbox is checked
function devmode_click() {
    disp_app_section($('#devmode').prop("checked"), 'start_local_gov_section');
}

// validate email address
function validateEmail(email) {
    var email_error = false;
    var re = /^(([^<>()[\]\\.,;:\s@\"]+(\.[^<>()[\]\\.,;:\s@\"]+)*)|(\".+\"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
    if (email && email.trim() && !re.test(email)) {
        email_error = true;
    }
    change_display(email_error, 'email', 'emailError');
    return !email_error;
}

// validate latitude, range (-90, 90), can be empty
function validateLatitude(value) {
    var lat_error = false;
    if (value) {
        if (!isReal(value)) {
            lat_error = true;
        } else {
            var valueFloat = parseFloat(value);
            if ((valueFloat > 90.00) || (valueFloat < -90.00)) {
                lat_error = true;
            }
        }
    }
    change_display(lat_error, 'lat', 'latError');
    return !lat_error;
}

// validate longitude, range (-180, 180), can be empty
function validateLongitude(value) {
    var lon_error = false;
    if (value) {
        if (!isReal(value)) {
            lon_error = true;
        } else {
            var valueFloat = parseFloat(value);
            if ((valueFloat > 180.00) || (valueFloat < -180.00)) {
                lon_error = true;
            }
        }
    }
    change_display(lon_error, 'lon', 'lonError');
    return !lon_error;
}

// expand or collapse the text when clicked
$("#location_notes_expand").click(function() {
    header = document.getElementById('location_notes_expand');
    details = document.getElementById('location_notes');
    if (header.innerHTML == "Collapse") {
        header.innerHTML = "Click for More...";
        details.style.display = "none";
    } else {
        header.innerHTML = "Collapse";
        details.style.display = "block";
    }
});

$(document).ready(function() {
    $('#localIp').text(document.domain);
});

$('#next').click(function() {
    // has to check again because it might be just switched from another page and lost
    // the red backgroud and error text for the invalid fields.
    var error_fields = "";
    if (!validateEmail($('#email').val())) {
        error_fields += "  Email\n";
    }
    if (!validateLatitude($('#lat').val())) {
        error_fields += "  Device Location/Latitude\n";
    }
    if (!validateLongitude($('#lon').val())) {
        error_fields += "  Device Location/Longitude\n";
    }
    if (error_fields && error_fields.length > 0) {
        alert("Please make sure the following fields have correct values:\n" + error_fields)
        return false;
    }

    //save the user input into the session storage so that it can be accessed in next page
    sessionStorage.setItem("lat", $('#lat').val());
    sessionStorage.setItem("lon", $('#lon').val());

    if ($('#gps').prop("checked")) {
        sessionStorage.setItem("gps", "1");
    } else {
        sessionStorage.setItem("gps", "0");
    }

    sessionStorage.setItem("email", $('#email').val());

    sessionStorage.setItem("start_local_gov", "0");
    if ($('#devmode').prop("checked")) {
        sessionStorage.setItem("devmode", "1");
        if ($('#start_local_gov').prop("checked")) {
            sessionStorage.setItem("start_local_gov", "1");
        }
    } else {
        sessionStorage.setItem("devmode", "0");
    }

    window.location = '/registration/application.html';
});
