import {
  auth,
  googleAuthProvider,
  twitterAuthProvider,
} from "../config/firebase";

import {
  Google as GoogleIcon,
  Twitter as TwitterIcon,
} from "@mui/icons-material";
import { Button, Grid, Typography } from "@mui/material";
import { useUser } from "../config/user.context";
import { useRouter } from "next/router";

export default function AuthPage() {
  const { user } = useUser();
  const router = useRouter();

  if (user) return router.push("/");
  return (
    <main>
      <Grid
        container
        spacing={2}
        direction="column"
        justifyContent="center"
        alignItems="center"
      >
        <Grid item>
          <Typography variant="h6">
            Please sign with one of the following providers
          </Typography>
        </Grid>
        <Grid item>
          <GoogleSignInButton />
        </Grid>
        <Grid item>
          <TwitterSignInButton />
        </Grid>
      </Grid>
    </main>
  );
}

// Sign in with Google button
function GoogleSignInButton() {
  const signInWithGoogle = async () => {
    await auth.signInWithPopup(googleAuthProvider);
  };

  return (
    <Button
      variant="contained"
      onClick={signInWithGoogle}
      startIcon={<GoogleIcon />}
    >
      Sign in with Google
    </Button>
  );
}

function TwitterSignInButton() {
  const signInWithTwitter = async () => {
    await auth.signInWithPopup(twitterAuthProvider);
  };

  return (
    <Button
      variant="contained"
      onClick={signInWithTwitter}
      startIcon={<TwitterIcon />}
    >
      Sign in with Twitter
    </Button>
  );
}
