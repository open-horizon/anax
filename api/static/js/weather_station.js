/* The JavaScript that is used in weather_station.html for device registration */
// redirect if contracts already in db
$.ajax({
    url: '/contract',
    type: 'GET',
    crossDomain: true,
    dataType: 'json',
    success: function(data) {
        if (data.length > 0) {
            window.location.href = "http://" + location.hostname + ":8000/";
        }
    },
});

function getNeighborhoodName() {
    var saved_lat = sessionStorage.getItem('lat');
    var saved_lon = sessionStorage.getItem('lon');
    var retry = 0;

    function doQuery() {
        $.ajax({
            url: 'http://maps.googleapis.com/maps/api/geocode/json?latlng=' + saved_lat + "," + saved_lon + "&sensor=false",
            type: 'GET',
            crossDomain: true,
            dataType: 'json',
            success: function(data) {
                if (data['status'] == "OK") {
                    // results is an array of location specifications in order of administrative levels.
                    // we will display the lowest level as long as its length is less than 32 bytes and
                    // it is not a street address.
                    for (value of data['results']) {
                        var addr = value["formatted_address"].trim();
                        if (addr.length <= 32) {
                            if (!value['types'].includes("street_address")) {
                                document.getElementById('pws_desc').value = addr;
                                break;
                            }
                        }
                    }
                } else {
                    document.getElementById('name_error').style.display = "block";
                    document.getElementById('pws_desc').value = "PWS " + saved_lat + "," + saved_lon;
                    console.log("Resolving geocode. result=" + JSON.stringify(data))
                }
            },
            error: function(xhr, status, err) {
                console.log("Error resolving geocode: " + status + err);
                retry++;
                if (retry < 5) {
                    setTimeout(doQuery, 1000);
                } else {
                    document.getElementById('pws_desc').value = "PWS " + saved_lat + "," + saved_lon;
                }
            },
        });
    }
    doQuery();
}

var saved_desc = sessionStorage.getItem("wugname");
if (saved_desc && saved_desc !== '') {
    document.getElementById('pws_desc').value = saved_desc;
}

var pws_selections = ["AcuRite 1025* (AcuRite)",
    "AcuRite 1035  (AcuRite)",
    "AcuRite 1036* (AcuRite)",
    "AcuRite 1525* (AcuRite)",
    "AcuRite 2032* (AcuRite)",
    "AcuRite 2064* (AcuRite)",
    "Aercus WS2083* (FineOffsetUSB)",
    "Aercus WS3083* (FineOffsetUSB)",
    "Ambient Weather WS1090* (FineOffsetUSB)",
    "Ambient Weather WS2080* (FineOffsetUSB)",
    "Ambient Weather WS2080A (FineOffsetUSB)",
    "Ambient Weather WS2090* (FineOffsetUSB)",
    "Ambient Weather WS2095* (FineOffsetUSB)",
    "Elecsa 6975*  (FineOffsetUSB)",
    "Elecsa 6976*  (FineOffsetUSB)",
    "Fine Offset WH1080* (FineOffsetUSB)",
    "Fine Offset WH1081* (FineOffsetUSB)",
    "Fine Offset WH1091* (FineOffsetUSB)",
    "Fine Offset WH1090* (FineOffsetUSB)",
    "Fine Offset WS1080* (FineOffsetUSB)",
    "Fine Offset WA2080  (FineOffsetUSB)",
    "Fine Offset WA2081* (FineOffsetUSB)",
    "Fine Offset WH2080* (FineOffsetUSB)",
    "Fine Offset WH2081* (FineOffsetUSB)",
    "Maplin N96GY* (FineOffsetUSB)",
    "Maplin N96FY* (FineOffsetUSB)",
    "National Geographic 265* (FineOffsetUSB)",
    "Oregon Scientific WMR88*   (WMR100)",
    "Oregon Scientific WMR88A   (WMR100)",
    "Oregon Scientific WMR100*  (WMR100)",
    "Oregon Scientific WMR100N* (WMR100)",
    "Oregon Scientific WMR180*  (WMR100)",
    "Oregon Scientific WMR180A* (WMR100)",
    "Sinometer WS1080 / WS1081*",
    "Sinometer WS3100 / WS3101*",
    "Tycon TP1080WC*  (FineOffsetUSB)",
    "Watson W-8681*   (FineOffsetUSB)",
    "Watson WX-2008*  (FineOffsetUSB)",
    "Velleman WS3080* (FineOffsetUSB)",
];
// fill the dropdown list with the values from above array
var selection_box = document.getElementById('pws_model_type');
for (var i = 0; i < pws_selections.length; i++) {
    var item = document.createElement('option');
    item.innerHTML = pws_selections[i];
    item.value = pws_selections[i];
    selection_box.appendChild(item);
}
// default selection
var saved_pws_model_type = sessionStorage.getItem("pws_model_type");
if (saved_pws_model_type && saved_pws_model_type !== '') {
    selection_box.value = saved_pws_model_type;
} else {
    selection_box.value = "Ambient Weather WS2080A (FineOffsetUSB)";
}

// back button is clicked
$('#back').click(function() {
    sessionStorage.setItem("wugname", $('#pws_desc').val());
    sessionStorage.setItem("pws_model_type", $('#pws_model_type').val());
    window.history.back();
});

// next button is clicked
$('#next').click(function() {
    // separate model and type
    var pws_model = "";
    var pws_st_type = "";
    var pws_model_type = $('#pws_model_type').val();
    if (pws_model_type.includes('(')) {
        var matches = pws_model_type.match("([^(]*)\\\((.*)\\\)");
        pws_model = matches[1].trim();
        pws_st_type = matches[2].trim();
    } else {
        pws_model = pws_model_type;
        pws_st_type = "";
    }
    // remove the last * from the pws_model
    if (/^.*\*$/.test(pws_model)) {
        pws_model = pws_model.slice(0, -1);
    }

    // save the user input into the session storage so that it can be accessed in next page
    sessionStorage.setItem("wugname", $('#pws_desc').val());
    sessionStorage.setItem("pws_model", pws_model.trim());
    sessionStorage.setItem("pws_st_type", pws_st_type.trim());
    sessionStorage.setItem("pws_model_type", pws_model_type.trim());



    // Move to next html page
    if (sessionStorage.getItem("citygram") == 1) {

        // If PWS was also clicked, go to that page
        window.location = '/registration/nyu_citygram.html';
    } else {
        window.location = '/registration/confirmation.html';
    }
});
