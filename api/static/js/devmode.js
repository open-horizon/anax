/* The JavaScript that is used in devmode.html for device registration */
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

// save all the data
function save_data() {
    sessionStorage.setItem("devmode_cmbhost", $('#devmode_cmbhost').val());
    sessionStorage.setItem("devmode_vinterval", $('#devmode_vinterval').val());
}

// validate data
function validate_data(id, id_error) {
    var value = document.getElementById(id).value;
    if (!value || !value.trim()) {
        change_display(true, id, id_error);
        return false
    } else {
        change_display(false, id, id_error);
        return true
    }
}

// validate the verification interval
function validate_vinterval(value) {
    var error = false;
    var valueInt = parseInt(value);
    if (!value || !value.trim() || !isPositiveInteger(value)) {
        error = true;
    }
    change_display(error, 'devmode_vinterval', 'devmode_vinterval_error');
    return !error;
}


// back button is clicked
$('#back').click(function() {
    save_data()
    window.history.back();
});

// next button is clicked
$('#next').click(function() {
    var status = true;
    if (!validate_data('devmode_cmbhost', 'devmode_cmbhost_error')) {
        status = false
    }
    if (!validate_vinterval($('#devmode_vinterval').val())) {
        status = false
    }

    if (status) {
        save_data()
        window.location = '/registration/confirmation.html';
    }
});
