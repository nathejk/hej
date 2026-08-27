<script setup lang="ts">
// Explains what the app records and why (PRD 002 §11.1, task 085).
//
// Exists because the location permission is no longer just "draw a dot on the map": the
// route is recorded and sent to the organizers. The pre-prompt has to stay short enough to
// read on a phone at 22:00, so the full version lives here and is linked from it — and from
// the profile page once PRD 003 lands.
//
// Written to be readable by a 12-year-old and by their parent. Deliberately plain Danish, no
// legalese: a text nobody reads is not consent, and this is the only written account most
// parents will see, since the start-area briefing reaches the participants and not them.
import { onMounted } from 'vue'
import { MapPin, Camera, ShieldCheck, Clock, Route } from '@lucide/vue'
import { useTrackStore } from '@/stores/track.store'

// The recording status below is here rather than in a developer tool on purpose
// (task 082): "storage growth is bounded or at least observable", and the person with
// the most right to see what the app has stored about them is the user. It also makes
// the two things that are otherwise invisible — whether the browser granted persistent
// storage, and whether writing has stopped — answerable instead of assumed.
const track = useTrackStore()
onMounted(() => void track.refreshCount())
</script>

<template>
  <div class="space-y-6 pb-4">
    <header>
      <h1 class="font-nathejk text-2xl text-slate-900">Data og privatliv</h1>
      <p class="mt-1 text-sm text-slate-500">
        Hvad appen gemmer om dig, og hvorfor. Du får også en gennemgang i startområdet — spørg
        endelig, hvis noget er uklart.
      </p>
    </header>

    <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
      <div class="flex items-start gap-3">
        <MapPin class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />
        <div>
          <h2 class="font-medium text-slate-800">Din placering</h2>
          <p class="mt-1 text-sm text-slate-600">
            Når du har givet lov, viser appen dig på kortet — og undervejs gemmer den din
            rute. Ruten sendes til Nathejks arrangører, så vi kan hjælpe jer, hvis I bliver i
            tvivl om, hvor I er, og så vi bagefter kan vise jer jeres egen rute.
          </p>
          <p class="mt-2 text-sm text-slate-600">
            Ruten optages kun, mens appen er åben. Ligger telefonen i lommen med skærmen
            slukket, holder optagelsen pause — så får ruten huller, og det er helt normalt.
          </p>
          <p class="mt-2 text-sm text-slate-600">
            Du kan altid slå placering fra igen i telefonens indstillinger. Så virker kortet
            stadig — du kan bare ikke se dig selv på det.
          </p>
        </div>
      </div>
    </section>

    <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
      <div class="flex items-start gap-3">
        <Camera class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />
        <div>
          <h2 class="font-medium text-slate-800">Dit billede</h2>
          <p class="mt-1 text-sm text-slate-600">
            Du bliver bedt om et billede af dit ansigt. Det bruges under løbet, så I kan se,
            hvem I møder i mørket — store dele af Nathejk foregår om natten, hvor ansigter er
            svære at genkende.
          </p>
          <p class="mt-2 text-sm text-slate-600">
            Billedet vises kun til andre, der er med i løbet, og deles ikke uden for Nathejk.
          </p>
        </div>
      </div>
    </section>

    <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
      <div class="flex items-start gap-3">
        <ShieldCheck class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />
        <div>
          <h2 class="font-medium text-slate-800">Dine oplysninger</h2>
          <p class="mt-1 text-sm text-slate-600">
            Appen viser de oplysninger, I allerede har givet ved tilmeldingen — navn,
            patrulje eller klan, og for spejdere nummeret på en voksen, vi kan ringe til
            under løbet. Du logger ind med dit eget telefonnummer.
          </p>
          <p class="mt-2 text-sm text-slate-600">
            Du kan ikke logge ind med en voksens nummer, og en voksen kan ikke logge ind som
            dig.
          </p>
        </div>
      </div>
    </section>

    <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
      <div class="flex items-start gap-3">
        <Route class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />
        <div>
          <h2 class="font-medium text-slate-800">Hvad ligger på din telefon lige nu?</h2>
          <p class="mt-1 text-sm text-slate-600">
            Ruten gemmes først på telefonen, og sendes videre når der er signal. Lige nu er
            der
            <strong>{{ track.pointCount }}</strong>
            {{ track.pointCount === 1 ? 'punkt' : 'punkter' }} gemt.
          </p>
          <p v-if="track.recording" class="mt-2 text-sm text-slate-600">
            Appen optager netop nu — der kommer et nyt punkt hvert halve minut, så længe appen
            er åben.
          </p>
          <p v-if="track.problem === 'full'" class="mt-2 text-sm text-amber-800">
            Der er ikke plads til flere punkter på telefonen, så optagelsen er stoppet. Sig
            det til en voksen fra Nathejk.
          </p>
          <p v-else-if="track.problem === 'capped'" class="mt-2 text-sm text-amber-800">
            Optagelsen er stoppet, fordi der er gemt usædvanligt mange punkter. Sig det til en
            voksen fra Nathejk.
          </p>
          <p v-if="!track.persisted" class="mt-2 text-sm text-slate-500">
            Telefonen har ikke lovet at beholde det, hvis der bliver pladsmangel. Derfor sender
            appen ruten videre, så snart den kan.
          </p>
          <RouterLink
            :to="{ name: 'track-status' }"
            class="mt-3 inline-block text-sm text-slate-600 underline"
          >
            Se teknisk sporingsstatus
          </RouterLink>
        </div>
      </div>
    </section>

    <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
      <div class="flex items-start gap-3">
        <Clock class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />
        <div>
          <h2 class="font-medium text-slate-800">Hvor længe gemmer vi det?</h2>
          <p class="mt-1 text-sm text-slate-600">
            Ruterne gemmer vi indtil videre. Det er første år, vi prøver det, og vi vil se,
            om det er noget, vi skal blive ved med — så vi har ikke sat en slutdato endnu.
          </p>
          <p class="mt-2 text-sm text-slate-600">
            Vil du have din rute eller dit billede slettet, så skriv til os, så gør vi det.
          </p>
        </div>
      </div>
    </section>
  </div>
</template>
