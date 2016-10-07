/* The JavaScript that is used in nyu_citygram.html for device registration */
// redirect if contracts already in db
$.ajax({
    url: '/contract',
    type: 'GET',
    crossDomain: true,
    dataType: 'json',
    success: function(data) {
        if (data.length > 0) {
            window.location.href = 'http://' + location.hostname + ':8000/';
        }
    },
});

// validate email address
function validateCGEmail(email) {
    undisplayErrorLabels();
    var err = false;
    var re = /^(([^<>()[\]\\.,;:\s@\"]+(\.[^<>()[\]\\.,;:\s@\"]+)*)|(\".+\"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
    if (email == '' || email && email.trim() && !re.test(email)) {
        err = true;
    }
    change_display(err, 'cg_email_inp', 'cg_emailError');
    return !err;
}

function checkCGPass(pass) {
    undisplayErrorLabels();
    var err = false;
    if (pass == '') {
        err = true;
    }
    change_display(err, 'cg_pass_inp', 'cg_passError');
    return !err;
}

function checkCGRSDName(rsdname) {
    undisplayErrorLabels();
    var err = false;
    if (rsdname == '') {
        err = true;
    }
    change_display(err, 'cg_rsdname_inp', 'cg_rsdnameError');
    return !err;
}

function undisplayErrorLabels() {
    var labels = ['pass_error', 
                'rsdname_error', 
                'general_error'];  
    for (label of labels) {
        document.getElementById(label).style.display = 'none';
    }
}

function registerCGRSD() {
    var saved_lat = sessionStorage.getItem('lat');
    var saved_lon = sessionStorage.getItem('lon');
    var cg_email = document.getElementById('cg_email_inp').value;
    var cg_pass = document.getElementById('cg_pass_inp').value;
    var cg_rsdname = document.getElementById('cg_rsdname_inp').value;
    var cg_rsddesc = document.getElementById('cg_rsddesc_inp').value;
    var retry = 0; 

    reg_data = {
        'email': cg_email, 
        'password': cg_pass, 
        'rsdname': cg_rsdname, 
        'rsddesc': cg_rsddesc, 
        'lat': saved_lat, 
        'lng': saved_lon
    }

    function doQuery() {
        $.ajax({
            url: 'https://citygramsound.com:4347/ibm_signin',
            type: 'POST',
            data: JSON.stringify(reg_data),
            contentType: 'application/json',
            crossDomain: true,
            dataType: 'json',
            //complete: callback,

            success: function(data) {
                if (data['status'].toUpperCase() == 'OK') {

                    // Possible OK responses:
                    // {'status':'ok','msg':'SIGNED UP'}   (new account created, sensor added)
                    // {'status':'ok','msg':'SIGNED IN'}   (email / pwd ok, RSD exists)
                    // {'status':'ok','msg':'ADDED RSD'}   (email / pwd ok, RSD added (desc opt))
                    console.log('Ciygram POST successful. result=' + JSON.stringify(data))

                    // Save data and move to next html page
                    saveAndContinue();

                } else if (data['status'].toUpperCase() == 'ERROR') {
                    var msg = data['msg'].toUpperCase();

                    // Possible error responses:
                    // {'status':'error','msg':'NOT MATCHED PASSWORD'} (email ok, bad pwd)
                    // {'status':'error','msg':'NO RSD'}        (no RSD provided in query) 
                    if (msg == 'NOT MATCHED PASSWORD' || msg == 'NO PASSWORD') {
                        document.getElementById('pass_error').style.display = 'block';
                    } else if (msg != 'NO RSD') {
                        document.getElementById('general_error').style.display = 'block';
                    }
                    console.log('Error on Ciygram signin. result=' + JSON.stringify(data))
                }
            },
            error: function(xhr, status, err) {
                console.log('Error registering account and RSD with Citygram: ' + status + err);
                document.getElementById('general_error').style.display = 'block';
                retry++;
                if (retry < 5) {
                    setTimeout(doQuery, 1000);
                } 
            },
        });
    }
    doQuery();
}

var saved_email = sessionStorage.getItem('cg_email');
if (saved_email && saved_email !== '') {
    document.getElementById('cg_email_inp').value = saved_email;
}
var saved_pass = sessionStorage.getItem('cg_pass');
if (saved_pass && saved_pass !== '') {
    document.getElementById('cg_pass_inp').value = saved_pass;
}
var saved_rsdname = sessionStorage.getItem('cg_rsdname');
if (saved_rsdname && saved_rsdname !== '') {
    document.getElementById('cg_rsdname_inp').value = saved_rsdname;
}
var saved_rsddesc = sessionStorage.getItem('cg_rsddesc');
if (saved_rsddesc && saved_rsddesc !== '') {
    document.getElementById('cg_rsddesc_inp').value = saved_rsddesc;
}

// back button is clicked
$('#back').click(function() {
    sessionStorage.setItem('cg_email', $('#cg_email_inp').val().trim());
    sessionStorage.setItem('cg_pass', $('#cg_pass_inp').val().trim());
    sessionStorage.setItem('cg_rsdname', $('#cg_rsdname_inp').val().trim());
    sessionStorage.setItem('cg_rsddesc', $('#cg_rsddesc_inp').val().trim());
    window.history.back();
});

// next button is clicked
$('#next').click(function() {
    
    undisplayErrorLabels();

    registerCGRSD();
});

function saveAndContinue() {
    // This function is run upon successful execution of registerCGRSD
    // save the user input into the session storage so that it can be accessed in next page
    sessionStorage.setItem('cg_email', $('#cg_email_inp').val().trim());
    sessionStorage.setItem('cg_pass', $('#cg_pass_inp').val().trim());
    sessionStorage.setItem('cg_rsdname', $('#cg_rsdname_inp').val().trim());
    sessionStorage.setItem('cg_rsddesc', $('#cg_rsddesc_inp').val().trim());

    // Move to next html page
    window.location = '/registration/confirmation.html';    
}
