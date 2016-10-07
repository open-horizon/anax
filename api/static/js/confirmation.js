/* The JavaScript that is used in confirmation.html for device registration */
// Display the device location on the map for user to varify.
var lat = sessionStorage.getItem("lat");
var lon = sessionStorage.getItem("lon");

// TODO: trim all strings that could contain whitespace, don't use integers as booleans, use === when comparing

//get general data
var hourly_cost_bacon = "120";
var gps = sessionStorage.getItem("gps");
var email = sessionStorage.getItem("email");
var devmode = sessionStorage.getItem("devmode");

// get the apps that are shown
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
var show_ipfs = devmode == "1" ? config.show_applications_devmode.ipfs :
    config.show_applications.ipfs;

//apps
var sdr = sessionStorage.getItem("sdr");
var citygram = sessionStorage.getItem("citygram");
var is_bandwidth_test_enabled = sessionStorage.getItem(
    "is_bandwidth_test_enabled");
var pws = sessionStorage.getItem("pws");
var cpu_temp = sessionStorage.getItem("cpu_temp");
var airpo = sessionStorage.getItem("airpo");
var ipfs = sessionStorage.getItem("ipfs");

disp_map(lat, lon);
disp_general_data();
disp_app_data();

// show the geneal data collected from user input
function disp_general_data() {
    var table = document.getElementById("tab_general_data");

    insert_row(table, "Latitude:", lat);
    insert_row(table, "Longitude:", lon);

    if (gps == "1") {
        insert_row(table, "GPS Kit Attached:", "yes");
    } else {
        insert_row(table, "GPS Kit Attached:", "no");
    }

    insert_row(table, "Email:", email);

    if (devmode == "1") {
        var start_local_gov = sessionStorage.getItem("start_local_gov");
        var local_gov_text = (start_local_gov == 1) ? "yes" : "no";
        insert_row(table, "Development Mode:", "yes");
        insert_row(table, "Start Local AgreementBot:", local_gov_text);
    } else {
        insert_row(table, "Development Mode:", "no");
    }
}

// insert a row with two cells for the given table.
function insert_row(table, text1, text2) {
    var row = table.insertRow(-1);
    var cell1 = row.insertCell(0);
    var cell2 = row.insertCell(1);
    cell1.innerHTML = text1;
    cell2.innerHTML = text2;
}

// display the application related data collected from user input.
function disp_app_data() {
    var table = document.getElementById("tab_app_data");
    var sep = "&nbsp;&nbsp;";
    if (show_netspeed) {
        show_netspeed_data(table);
        insert_row(table, sep, "")
    }

    if (show_citygram) {
        show_citygram_data(table);
        insert_row(table, sep, "")
    }

    if (show_sdr) {
        show_sdr_data(table);
        insert_row(table, sep, "")
    }

    if (show_weather_station) {
        show_pws_data(table);
        insert_row(table, sep, "")
    }

    if (show_cpu_temp) {
        show_cpu_temp_data(table);
        insert_row(table, sep, "")
    }

    if (show_air_pollution) {
        show_airpo_data(table);
        insert_row(table, sep, "")
    }

    if (show_ipfs) {
        show_ipfs_data(table);
        insert_row(table, sep, "")
    }
}

// display netspeed application settings
function show_netspeed_data(table) {
    var netspeed_app = "Netspeed";
    if (is_bandwidth_test_enabled == "1") {
        var target_server = sessionStorage.getItem(
            "bandwidth_test_target_server");
        var sep = "&nbsp;&nbsp;&nbsp;&nbsp;";
        insert_row(table, netspeed_app, "Yes");
        insert_row(table, sep + "Target test server", target_server);
    } else {
        insert_row(table, netspeed_app, "No");
    }
}

// display sdr application settings
function show_sdr_data(table) {
    var sdr_app = "SDR (Software-Defined Radio)";
    if (sdr == "1") {
        insert_row(table, sdr_app, "Yes");
    } else {
        insert_row(table, sdr_app, "No");
    }
}

// display citygram application settings
function show_citygram_data(table) {
    var citygram_app = "Citygram";
    if (citygram == "1") {
        var sep = "&nbsp;&nbsp;&nbsp;&nbsp;";
        insert_row(table, citygram_app, "Yes");
        insert_row(table, sep + "Account email:",
            sessionStorage.getItem("cg_email"));
        insert_row(table, sep + "Account password:",
            sessionStorage.getItem("cg_pass"));
        insert_row(table, sep + "Sensor name:",
            sessionStorage.getItem("cg_rsdname"));
        insert_row(table, sep + "Sensor description (opt):",
            sessionStorage.getItem("cg_rsddesc"));
    } else {
        insert_row(table, citygram_app, "No");
    }
}

// display weather station application settings
function show_pws_data(table) {
    var pws_app = "Personal Weather Station";
    if (pws == "1") {
        var sep = "&nbsp;&nbsp;&nbsp;&nbsp;";
        insert_row(table, pws_app, "Yes");
        insert_row(table, sep + "Weather Station Description:",
            sessionStorage.getItem("wugname"));
        insert_row(table, sep + "Weather Station Model:", sessionStorage.getItem(
            "pws_model"));
        insert_row(table, sep + "Weather Station Type:", sessionStorage.getItem(
            "pws_st_type"));

    } else {
        insert_row(table, pws_app, "No");
    }
}

// display cpu temperature application settings
function show_cpu_temp_data(table) {
    var cpu_temp_app = "CPU Temperature Sharing";
    if (cpu_temp == "1") {
        insert_row(table, cpu_temp_app, "Yes");
    } else {
        insert_row(table, cpu_temp_app, "No");
    }

}

// display air pollution application settings
function show_airpo_data(table) {
    var airpo_app = "Air Pollution";
    if (airpo == "1") {
        var sep = "&nbsp;&nbsp;&nbsp;&nbsp;";
        insert_row(table, airpo_app, "Yes");
        insert_row(table, sep + "PurpleAir sensor name:", sessionStorage.getItem(
            "airpo_sensor_name"));
    } else {
        insert_row(table, airpo_app, "No");
    }
}

// display ipfs application settings
function show_ipfs_data(table) {
    var ipfs_app = "IPFS (Beta)";
    if (ipfs == "1") {
        insert_row(table, ipfs_app, "Yes");
    } else {
        insert_row(table, ipfs_app, "No");
    }
}

// Diaply the GPS coordinates on the map
function disp_map(lat, lon) {
    if (lat && lat.trim() && lon && lon.trim()) {
        document.getElementById('map').src =
            "http://staticmap.openstreetmap.de/staticmap.php?center=" +
             + lat + "," + lon + "&zoom=15&size=600x300&markers=" +
             + lat + "," + lon + ",ol-marker";
        document.getElementById('map').style.visibility = 'visible';
        document.getElementById('review_loc_text').style.display = 'block';
        document.getElementById('no_location').style.display = 'none';
    } else {
        document.getElementById('map').style.visibility = 'hidden';
        document.getElementById('review_loc_text').style.display = 'none';
        document.getElementById('no_location').style.display = 'block';
    }
}

// Pass iotf conf to the anax server so that it will create policy files for the governor for devmoce.
function set_iotf_conf() {
    var quarks_conf = {
        cloudMsgBrokerHost: null,
        dataVerificationInterval: null,
    }

    var name;
    if (is_bandwidth_test_enabled == "1") {
        name = 'netspeed'
    } else if (sdr == "1") {
        name = 'sdr'
    } else if (pws == "1") {
        name = 'pws'
    } else if (cpu_temp == "1") {
        name = 'cpu_temp'
    }

    iotf_conf = {
        'name': name,
        'apiSpec': [{
            'specRef': 'https://bluehorizon.network/documentation/' +
                name + '-device-api/'
        }],
        'arch': 'arm',
        'quarks': quarks_conf
    }

    $.ajax({
        url: '/iotfconf',
        type: 'POST',
        data: JSON.stringify(iotf_conf),
        crossDomain: true,
        contentType: 'application/json',
        success: function(data) {},
        error: function(xhr, status, err) {}
    });
}

// create a sdr contract object
function sdr_contract() {
    var pay = {
        'name': 'SDR Contract',
        'hourly_cost_bacon': parseInt(hourly_cost_bacon) * 2,
        'app_attributes': {
            'sdr': "true"
        }
    }
    return pay;
}

// create a Citygram contract object
function citygram_contract() {
    var cg_email = sessionStorage.getItem("cg_email");
    var cg_pass = sessionStorage.getItem("cg_pass");
    var cg_rsdname = sessionStorage.getItem("cg_rsdname");
    var cg_rsddesc = sessionStorage.getItem("cg_rsddesc");

    var pay = {
        'name': 'Citygram Contract',
        'hourly_cost_bacon': parseInt(hourly_cost_bacon) * 2,
        'ram': 256,
        'app_attributes': {
          "citygram": 'true'
        },
        "private_app_attributes": {
          "cg_email": cg_email,
          "cg_pass": cg_pass,
          "cg_rsdname": cg_rsdname,
          "cg_rsddesc": cg_rsddesc
        }
    }
    return pay;
}

// create a netspeed contract object
function netspeed_contract() {
    var bandwidth_test_target_server = sessionStorage.getItem(
        "bandwidth_test_target_server");
    var pay = {
        'name': 'Netspeed Contract',
        'hourly_cost_bacon': Math.round(parseInt(hourly_cost_bacon) *
            1.5),
        'app_attributes': {
            'is_bandwidth_test_enabled': "true",
            'target_server': bandwidth_test_target_server,
        },
    }
    return pay;
}

// create a weather station contract object
function pws_contract() {
    var wugname = sessionStorage.getItem("wugname");
    var pws_model = sessionStorage.getItem("pws_model");
    var pws_st_type = sessionStorage.getItem("pws_st_type");

    var pay_pws = {
        'name': 'PWS Contract',
        'hourly_cost_bacon': parseInt(hourly_cost_bacon) * 2,
        'app_attributes': {
            'pws': "true",
            'wugname': wugname,
            'pws_model': pws_model,
            'pws_st_type': pws_st_type,
        },
    };
    return pay_pws;
}

// create a cpu temperature contract object
function cpu_temp_contract() {
    var pay = {
        'name': 'CPU Temperature Contract',
        'hourly_cost_bacon': parseInt(hourly_cost_bacon) * 2,
        'app_attributes': {
            'cpu_temp': "true"
        }
    }
    return pay;
}

// create a air pollution contract object
function air_pollution_contract() {
    var airpo_sensor_name = sessionStorage.getItem("airpo_sensor_name");
    var pay = {
        'name': 'Air Pollution Contract',
        'hourly_cost_bacon': parseInt(hourly_cost_bacon) * 2,
        'app_attributes': {
            'air_pollution': "true",
            'purple_air_sensor_name': airpo_sensor_name,
        },
    };
    return pay;
}

// create a IPFS contract object
function ipfs_contract() {
    var pay = {
        'name': 'IPFS Contract',
        'hourly_cost_bacon': parseInt(hourly_cost_bacon) * 2,
        'app_attributes': {
            'ipfs': "true"
        }
    }
    return pay;
}

// create a location contract object
function location_contract() {
    if (gps == "1" && devmode != "1") {
        var pay = {
            'name': 'Location Contract',
            'hourly_cost_bacon': parseInt(hourly_cost_bacon),
            'app_attributes': {
                'gps': "true",
                'gpsdevice': "true"
            },
        }

        return pay;

    } else if (lat && lat.trim() && lon && lon.trim() && devmode != "1") {
        var pay = {
            'name': 'Location Contract',
            'hourly_cost_bacon': parseInt(hourly_cost_bacon),
            'app_attributes': {
                'gps': "true"
            }
        }

        return pay;
    }
    return null;
}

//go back to index.html when back button is clicked
$('#back').click(function() {
    window.history.back();
});

// send a request to the horizon server to set up development mode.
function set_devmode() {
    if (devmode == "1") {
        var start_local_gov = sessionStorage.getItem("start_local_gov");
        var localgov = (start_local_gov == 1) ? true : false;
        $.ajax({
            url: '/devmode',
            type: 'POST',
            data: JSON.stringify({
                "mode": true,
                "localgov": localgov
            }),
            crossDomain: true,
            contentType: 'application/json',
            success: function(data) {},
            error: function(xhr, status, err) {}
        });

        set_iotf_conf();
    } else {
        $.ajax({
            url: '/devmode',
            type: 'POST',
            data: JSON.stringify({
                "mode": false,
                "localgov": false
            }),
            crossDomain: true,
            contentType: 'application/json',
            success: function(data) {},
            error: function(xhr, status, err) {}
        });
    }
}

//handle submit button
$('#submit').click(function() {
    set_devmode();

    payloads = [];

    var add = function(obj) {
        if (!obj.hasOwnProperty('ram')) {
            obj['ram'] = 128;
        }

        if (!obj.hasOwnProperty('private_app_attributes')) {
          obj['private_app_attributes'] = {};
        }

        obj['private_app_attributes']['lat'] = lat;
        obj['private_app_attributes']['lon'] = lon;

        payloads.push(obj);
    }

    // create location contract
    var pay = location_contract();
    if (pay !== null) {
        add(pay);
    }

    //create other contracts
    if (sdr == "1") {
        add(sdr_contract())
    }

    if (citygram == "1") {
        add(citygram_contract())
    }

    if (is_bandwidth_test_enabled == "1") {
        add(netspeed_contract());
    }

    if (pws == "1") {
        add(pws_contract());
    }

    if (cpu_temp == "1") {
        add(cpu_temp_contract());
    }

    if (airpo == "1") {
        add(air_pollution_contract());
    }

    if (ipfs == "1") {
        add(ipfs_contract());
    }

    var needsRetry = false;
    var processed = [];
    sessionStorage.setItem("contract_submitted", "");

    payloads.forEach(function(payload) {
      $.ajax({
        url: '/contract',
        type: 'POST',
        data: JSON.stringify(payload),
        crossDomain: true,
        contentType: 'application/json'
      }).done(function(xhr) {
        console.log("Succeeded registering contract " + payload.name + ".");
      }).fail(function(xhr, tStatus, err) {
          console.log("Error POSTing contract", tStatus, err);
          if (xhr.status === 409) {
            // we're cool if it's 409, that means it was already submitted successfully with the same name. Will become a problem when we allow re-registration
            console.log("Ignoring this error b/c a user may have re-submitted form after correcting some contracts' input errors")
          } else {
            // these errors can't be ignored
            needsRetry = true;

            if (xhr.status === 503) {
            alert("The registration cannot procceed because the device is disconnected from the internet. Ensure the device is connected and submit again.");

            } else if (xhr.status === 400) {

            // try and parse out an error type
              var userError = JSON.parse(xhr.responseText);

            if (userError.hasOwnProperty('error') && userError.hasOwnProperty('label')) {
              // these aren't coded such that we can use them in processing, but we can give clues to a developer about problems
              console.log("Failed to POST contract named " + payload.name + ".", userError);
            }

            alert("Error registering " + payload.name + " because of an illegal field value (only plain text, spaces, numbers, and the special characters ', \", @, and () are permitted). Please correct the problem and submit again.");

            } else {
              alert("Error registering " + payload.name + ", please try again. If the problem persists, please report this error.")
            }
          }
      }).always(function() {
        processed.push(payload);

        // evaluate completion
        if (processed.length === payloads.length && !needsRetry) {
          if (email !== "") {
            // also submit account email if provided
            $.ajax({
                url: '/account',
                type: 'POST',
                data: JSON.stringify({
                    "email": email
                }),
                crossDomain: true,
                contentType: 'application/json'
            });
          }

          sessionStorage.setItem("contract_submitted", payloads.map(function(pay) { return pay.name; }).join(','));
          window.location = '/registration/status.html';
        }
      });
    });
});
