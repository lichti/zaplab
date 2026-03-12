// Auth section — handles dashboard login/logout and session state.
function authSection() {
  return {
    loginEmail:    '',
    loginPassword: '',
    oldPassword:     '',
    newPassword:     '',
    confirmPassword: '',
    mustChangePassword: false,
    authLoading:   false,
    authError:     null,

    profile: {
      email:    '',
      name:     '',
      avatar:   '',
      loading:  false,
      success:  false,
    },

    async initAuth() {
      // Listen for auth changes (e.g. from other tabs or manual logout)
      pb.authStore.onChange((token, model) => {
        this.isLoggedIn = pb.authStore.isValid;
        if (this.isLoggedIn) {
          this.checkMustChange();
          this.initProfile();
          this.authError = null;
          this.dashFetch?.(); // refresh dashboard when logging in
        }
      });

      // Give a tiny moment for the auth store to hydrate from localStorage
      await new Promise(r => setTimeout(r, 100));

      this.isLoggedIn = pb.authStore.isValid;

      // Verify session with server on startup if we think we are logged in
      if (this.isLoggedIn) {
        try {
          await pb.collection('users').authRefresh();
          this.checkMustChange();
          this.initProfile();
        } catch (err) {
          // ONLY logout if the error is 401 or 403 (unauthorized/forbidden)
          // If it's a network error or 500, keep the local session.
          if (err.status === 401 || err.status === 403) {
            console.warn('Session invalidated by server, logging out.');
            this.logout();
          } else {
            console.warn('Auth refresh failed (network/server error), keeping local session:', err);
            this.checkMustChange();
            this.initProfile();
          }
        }
      }
    },

    initProfile() {
      if (pb.authStore.model) {
        this.profile.email  = pb.authStore.model.email || '';
        this.profile.name   = pb.authStore.model.name  || '';
        this.profile.avatar = pb.authStore.model.avatar || '';
      }
    },

    async saveProfile() {
      this.profile.loading = true;
      this.profile.success = false;
      this.authError = null;
      try {
        await pb.collection('users').update(pb.authStore.model.id, {
          name:   this.profile.name,
          email:  this.profile.email,
        });
        this.profile.success = true;
        setTimeout(() => this.profile.success = false, 3000);
      } catch (err) {
        console.error('Profile update failed:', err);
        this.authError = err.message || 'Failed to update profile';
      } finally {
        this.profile.loading = false;
      }
    },

    checkMustChange() {
      if (pb.authStore.isValid && pb.authStore.model) {
        this.mustChangePassword = pb.authStore.model.force_password_change;
      } else {
        this.mustChangePassword = false;
      }
    },

    async login() {
      this.authLoading = true;
      this.authError   = null;
      try {
        const authData = await pb.collection('users').authWithPassword(this.loginEmail, this.loginPassword);
        this.loginPassword = '';
        this.mustChangePassword = authData.record.force_password_change;
      } catch (err) {
        console.error('Login failed:', err);
        this.authError = err.message || 'Invalid credentials';
      } finally {
        this.authLoading = false;
      }
    },

    async changePassword() {
      if (this.newPassword !== this.confirmPassword) {
        this.authError = 'Passwords do not match';
        return;
      }
      if (this.newPassword.length < 8) {
        this.authError = 'Password must be at least 8 characters';
        return;
      }

      this.authLoading = true;
      this.authError   = null;
      try {
        await pb.collection('users').update(pb.authStore.model.id, {
          oldPassword:           this.oldPassword,
          password:              this.newPassword,
          passwordConfirm:       this.confirmPassword,
          force_password_change: false,
        });
        
        // Re-authenticate to refresh the token and model
        await pb.collection('users').authWithPassword(this.loginEmail || pb.authStore.model.email, this.newPassword);
        
        this.mustChangePassword = false;
        this.oldPassword = '';
        this.newPassword = '';
        this.confirmPassword = '';
      } catch (err) {
        console.error('Password change failed:', err);
        // Extract detailed error from PocketBase ClientResponseError if available
        let detail = '';
        if (err.originalError && err.originalError.data) {
          detail = ': ' + JSON.stringify(err.originalError.data);
        } else if (err.data && err.data.data) {
          detail = ': ' + JSON.stringify(err.data.data);
        }
        this.authError = (err.message || 'Failed to change password') + detail;
      } finally {
        this.authLoading = false;
      }
    },

    promptChangePassword() {
      this.mustChangePassword = true;
      this.oldPassword = '';
      this.newPassword = '';
      this.confirmPassword = '';
      this.authError = null;
    },

    logout() {
      pb.authStore.clear();
      this.isLoggedIn = false;
    }
  };
}
