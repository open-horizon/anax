/* The JavaScript that is used in index.html for device registration */
//decides what applications to show
var devmode = sessionStorage.getItem("devmode");

var show_netspeed = devmode == "1" ? config.show_applications_devmode.netspeed :
    config.show_applications.netspeed;
var show_sdr = devmode == "1" ? config.show_applications_devmode.sdr :
    config.show_applications.sdr;
var show_citygram = devmode == "1" ? config.show_applications_devmode.citygram :
    config.show_applications.citygram;
var show_weather_station = devmode == "1" ? config.show_applications_devmode.weather_station :
    config.show_applications.weather_station;
var show_cpu_temp = devmode == "1" ? config.show_applications_devmode.cpu_temp :
    config.show_applications.cpu_temp;
var show_air_pollution = devmode == "1" ? config.show_applications_devmode.air_pollution :
    config.show_applications.air_pollution;
var show_ipfs = devmode == "1" ? config.show_applications_devmode.ipfs : config
    .show_applications.ipfs;

var app_count = 0;

// hind and show the application according to the config file setting
disp_app_section(show_netspeed, "ns_app_section");
disp_app_section(show_sdr, "sdr_app_section");
disp_app_section(show_citygram, "citygram_app_section");
disp_app_section(show_weather_station, "pws_app_section");
disp_app_section(show_cpu_temp, "cpu_temp_app_section");
disp_app_section(show_air_pollution, "airpo_app_section");
disp_app_section(show_ipfs, "ipfs_app_section");

init_display();

// initially enable and disable some fields
function init_display() {
    // show the details section if the application is checked   
    disp_app_section(document.getElementById('is_bandwidth_test_enabled').checked,
        'ns_app_setting');
    disp_app_section(document.getElementById('airpo').checked,
        'airpo_app_setting');

    // check the cpu temp by default for devmode
    if (devmode == "1") {
        document.getElementById('cpu_temp').checked = true;
    }
}

// expand the collapse the details sections when clicked
function expendClicked(header, id_detail) {
    details = document.getElementById(id_detail);
    if (header.innerHTML == "^ Less") {
        header.innerHTML = "> More...";
        details.style.display = "none";
    } else {
        header.innerHTML = "^ Less";
        details.style.display = "block";
    }
}

// show the setting section when an application is selected
function appClicked(cb, id_setting) {
    if (id_setting != 'none') {
        disp_app_section(cb.checked, id_setting);
    }

    if (cb.id == "airpo") {
        change_display(false, 'airpo_sensor_name', 'sensor_nameError');
    }
}

// validate PurpleAir sensor name 
function validatePurpleAirSensorName(value) {
    var sensor_name_error = false;
    if (!value || !value.trim()) {
        sensor_name_error = true;
    }
    change_display(sensor_name_error, 'airpo_sensor_name',
        'sensor_nameError');
    return !sensor_name_error;
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

// save the sdr application setting in the session storage
function save_sdr_data() {
    if ((show_sdr) && ($('#sdr').prop("checked"))) {
        app_count++;
        sessionStorage.setItem("sdr", "1");
    } else {
        sessionStorage.setItem("sdr", "0");
    }
    return 0;
}

// save the citygram application setting in the session storage
function save_citygram_data() {
    if ((show_citygram) && ($('#citygram').prop("checked"))) {
        app_count++;
        sessionStorage.setItem("citygram", "1");
    } else {
        sessionStorage.setItem("citygram", "0");
    }
    return 0;
}

// save the netspeed application setting in the session storage
function save_netspeed_data() {
    if ((show_netspeed) && ($('#is_bandwidth_test_enabled').prop("checked"))) {
        app_count++;
        sessionStorage.setItem("is_bandwidth_test_enabled", "1");
        sessionStorage.setItem("bandwidth_test_target_server", $(
            '#bandwidth_test_target_server').val());
    } else {
        sessionStorage.setItem("is_bandwidth_test_enabled", "0");
    }
    return 0;
}

// save the personal weather station application setting in the session storage
function save_pws_data() {
    if ((show_weather_station) && ($('#pws').prop("checked"))) {
        app_count++;
        sessionStorage.setItem("pws", "1");
    } else {
        sessionStorage.setItem("pws", "0");
    }
    return 0;
}

// save the cpu temperature application setting in the session storage
function save_cpu_temp_data() {
    if ((show_cpu_temp) && ($('#cpu_temp').prop("checked"))) {
        app_count++;
        sessionStorage.setItem("cpu_temp", "1");
    } else {
        sessionStorage.setItem("cpu_temp", "0");
    }
    return 0;
}

// save the air pollution application setting in the session storage
function save_air_pollution_data() {
    if ((show_air_pollution) && ($('#airpo').prop("checked"))) {
        app_count++;
        sessionStorage.setItem("airpo", "1");
        var sensor_name = $('#airpo_sensor_name').val();
        if (!validatePurpleAirSensorName(sensor_name)) {
            return 1;
        } else {
            sessionStorage.setItem("airpo_sensor_name", sensor_name);
        }
    } else {
        sessionStorage.setItem("airpo", "0");
    }
    return 0;
}

// save the ipfs application setting in the session storage
function save_ipfs_data() {
    if ((show_ipfs) && ($('#ipfs').prop("checked"))) {
        app_count++;
        sessionStorage.setItem("ipfs", "1");
    } else {
        sessionStorage.setItem("ipfs", "0");
    }
    return 0;
}

//go back to index.html when back button is clicked
$('#back').click(function() {
    window.history.back();
});

$('#next').click(function() {

    app_count = 0;
    if (save_sdr_data() == 1) {
        return false;
    }
    if (save_citygram_data() == 1) {
        return false;
    }
    if (save_netspeed_data() == 1) {
        return false;
    }
    if (save_pws_data() == 1) {
        return false;
    }
    if (save_cpu_temp_data() == 1) {
        return false;
    }
    if (save_air_pollution_data() == 1) {
        return false;
    }
    if (save_ipfs_data() == 1) {
        return false;
    }

    if ((devmode == "1") && (app_count != 1)) {
        disp_app_section(1, "devmode_check")
    } else {
        // decides next page
        if ((show_weather_station) && ($('#pws').prop("checked"))) {
            window.location = '/registration/weather_station.html';
            // when completed, weather_station.html forwards on to nyu_citygram or confirmation
        }
        else if ((show_citygram) && ($('#citygram').prop("checked"))) {
            window.location = '/registration/nyu_citygram.html';
            // when completed, nyu_citygram forwards on to confirmation
        } else {
            window.location = '/registration/confirmation.html';
        }
    }
});