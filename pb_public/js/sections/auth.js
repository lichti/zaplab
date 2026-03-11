// Auth section — handles dashboard login/logout and session state.
function authSection() {
  return {
    isLoggedIn:    pb.authStore.isValid,
    loginEmail:    '',
    loginPassword: '',
    authLoading:   false,
    authError:     null,

    initAuth() {
      // Listen for auth changes (e.g. from other tabs or manual logout)
      pb.authStore.onChange((token, model) => {
        this.isLoggedIn = pb.authStore.isValid;
        if (this.isLoggedIn) {
          this.authError = null;
          this.dashFetch(); // refresh dashboard when logging in
        }
      });
    },

    async login() {
      this.authLoading = true;
      this.authError   = null;
      try {
        await pb.collection('users').authWithPassword(this.loginEmail, this.loginPassword);
        this.loginPassword = '';
      } catch (err) {
        console.error('Login failed:', err);
        this.authError = err.message || 'Invalid credentials';
      } finally {
        this.authLoading = false;
      }
    },

    logout() {
      pb.authStore.clear();
      this.isLoggedIn = false;
    }
  };
}
