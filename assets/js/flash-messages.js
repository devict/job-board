document.addEventListener('DOMContentLoaded', function() {
    document.querySelectorAll('.flash-message .btn-close').forEach(function(button) {
        button.addEventListener('click', function(e) {
            e.preventDefault();
            this.closest('.flash-message').remove();
        });
    });
});