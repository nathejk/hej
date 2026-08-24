import type { Component } from 'vue'
import { Award, Backpack, Compass, Footprints, ShieldCheck } from '@lucide/vue'

/*
  The rulebook is static, offline-first content: it must be readable with no
  network (the service worker precaches the app bundle), so it lives here as data
  rather than behind a BFF endpoint. Structured as blocks — not a blob of HTML —
  so the view can render lists, callouts and phone links as real components with
  correct semantics and touch targets.

  Copy is the official Nathejk "Regler" text and is authoritative: change it only
  when løbsledelsen changes the rules.
*/

export type RulebookBlock =
  | { kind: 'text'; text: string }
  | { kind: 'subheading'; text: string }
  | { kind: 'list'; lead?: string; items: string[] }
  /** Highlighted note. `warning` is for rules with immediate consequences. */
  | { kind: 'callout'; tone: 'info' | 'warning'; title?: string; text: string }
  /** Callout with a tap-to-call action. `phone` is the dialable form. */
  | { kind: 'phone'; label: string; text: string; phone: string; display: string }

export interface RulebookSection {
  /** Stable id — used as the accordion item value and as a deep-link anchor. */
  id: string
  title: string
  /** One-line gist shown under the title while the section is collapsed. */
  summary: string
  icon: Component
  blocks: RulebookBlock[]
}

/** Døgnbemandet forældre-kontakttelefon — kun til alvorlige situationer. */
export const FORAELDRETELEFON = '+4581113119'
export const FORAELDRETELEFON_DISPLAY = '81 11 31 19'

export const rulebookIntro: RulebookBlock[] = [
  {
    kind: 'text',
    text: 'Nathejk er et adventurespejdløb, hvor spejderne skal klare sig selv fra start til slut. Derfor er det nødvendigt med nogle regler, som alle spejdere skal overholde. Løbsledelsen forbeholder sig ret til at tage spejdere og/eller patruljer, der ikke overholder reglerne, ud af løbet og sende dem hjem for egen regning inden løbet er slut.',
  },
  {
    kind: 'callout',
    tone: 'info',
    title: 'Den vigtigste regel på Nathejk',
    text: 'Nathejk må gerne være hårdt, men husk at det kun er en leg – alle skal have det sjovt!',
  },
]

export const rulebookSections: RulebookSection[] = [
  {
    id: 'sikkerhedsudstyr',
    title: 'Sikkerhedsudstyr',
    summary: 'Hvad hver spejder og hver patrulje skal have med hele løbet',
    icon: ShieldCheck,
    blocks: [
      {
        kind: 'list',
        lead: 'Nedenstående effekter betragtes som sikkerhedsudstyr, og skal medbringes til hver enkelt spejder på hele løbet:',
        items: [
          'Sovepose – du skal kunne ligge udstrakt i den',
          'Liggeunderlag – mindst samme længde som dig',
          'Regntøj bestående af både regnjakke og regnbukser',
          'Refleks (godkendt og synlig fra alle sider, f.eks. ankelrefleks)',
        ],
      },
      {
        kind: 'list',
        lead: 'Herudover skal hver patrulje medbringe:',
        items: [
          'Førstehjælpstaske (minimumsindhold: plaster, vabelplaster, kompresforbinding og støttebind)',
          'Kompas',
        ],
      },
      {
        kind: 'callout',
        tone: 'warning',
        text: 'Der vil blive foretaget stikprøvekontrol af sikkerhedsudstyr i start, undervejs i løbet og i mål. Manglende sikkerhedsudstyr kan medføre, at en spejder/patrulje tages ud af løbet.',
      },
    ],
  },
  {
    id: 'faerdsel',
    title: 'Færdsel',
    summary: 'Hvor I må gå og opholde jer – og hvor I aldrig må',
    icon: Footprints,
    blocks: [
      { kind: 'text', text: 'Alle skal overholde færdselsloven.' },
      {
        kind: 'list',
        lead: 'Herudover har Nathejk nogle særlige regler for færdsel, der også skal overholdes:',
        items: [
          'Færdsel og ophold på golfbaner, flyvepladser, på eller langs med motorveje, motortrafikveje og jernbaner er ikke tilladt',
          'Ophold og pauser på/ved jernbaneoverskæringer, broer og viadukter over og under motorveje, motortrafikveje og jernbaner er ikke tilladt',
          'Færdsel og ophold i private skove efter mørkets frembrud uden ejerens tilladelse er ikke tilladt',
          'Færdsel og ophold på dyrkede/nysåede arealer uden ejerens tilladelse er ikke tilladt',
          'Spejderne bevæger sig til fods. Brug af cykler, busser, tog, biler og andre transportformer er ikke tilladt',
          'Alle skal bære godkendte, synlige reflekser ved færdsel på og langs med veje. Reflekser skal medbringes hjemmefra',
          'I områder, der på det udleverede kort er markeret med rød markering, er færdsel og ophold ikke tilladt',
        ],
      },
      {
        kind: 'callout',
        tone: 'warning',
        title: 'Overtrædelse medfører øjeblikkelig hjemsendelse',
        text: 'Er du i tvivl om, hvilke veje der er forbudt område – eller hvilke regler der gælder – så kontakt nødtelefonen.',
      },
    ],
  },
  {
    id: 'maerker',
    title: 'Mærker',
    summary: 'Kravene til det hvide og det sorte Nathejk-mærke',
    icon: Award,
    blocks: [
      { kind: 'subheading', text: 'Det hvide Nathejk-mærke' },
      {
        kind: 'list',
        lead: 'Det hvide mærke udleveres til de spejdere, der gennemfører Nathejk og opfylder nedenstående:',
        items: [
          'Har været på alle check-points',
          'Kommer samlet i mål senest søndag morgen kl. 04.00',
        ],
      },
      { kind: 'subheading', text: 'Det sorte mærke' },
      {
        kind: 'list',
        lead: 'Det sorte mærke gives kun til patruljer, der gennemfører Nathejk og opfylder nedenstående:',
        items: [
          'Har besøgt alle check-points og poster inden for den åbningstid, der er trykt på kortet/kortene',
          'Ikke er blevet fanget undervejs i løbet',
          'Kommer samlet i mål senest kl. 04.00 søndag morgen',
          'Består af de samme medlemmer under hele løbet – og alle skal gennemføre',
        ],
      },
    ],
  },
  {
    id: 'andet',
    title: 'Andet',
    summary: 'Patruljen, påklædning, opførsel og udgang af løbet',
    icon: Compass,
    blocks: [
      {
        kind: 'text',
        text: 'En patrulje består af mindst tre og højst syv spejdere. Alle spejdere i patruljen skal være fyldt 12 år, og ingen må være fyldt 17 år den dag, Nathejk starter. Gennemsnitsalderen på patruljens medlemmer skal være mindst 13 år.',
      },
      {
        kind: 'text',
        text: 'Nathejk er åbent for spejdere fra alle korps, og patruljer kan sammensættes på tværs af korps.',
      },
      {
        kind: 'list',
        items: [
          'Vi repræsenterer spejderbevægelsen. Spejdertørklædet skal altid bæres synligt',
          'Militærudklædning og ansigtssløring er ikke tilladt for spejdere på Nathejk',
          'Jeres skrald skal altid tages med videre og smides i en skraldespand. Vi er spejdere, og vi rydder op efter os selv',
          'Nathejk er en fangeleg, men slåskampe og anden form for vold tolereres på ingen måde',
          'Patruljen skal transportere egen oppakning under hele løbet',
          'Alkohol er strengt forbudt på Nathejk',
        ],
      },
      {
        kind: 'text',
        text: 'Når en spejder eller en patrulje først er taget ud af løbet og kørt til Nathejks hovedkvarter, så er løbet slut for vedkommende.',
      },
      {
        kind: 'text',
        text: 'Hvis patruljen ikke når en post/check-point inden for åbningstiden, skal nødtelefonen kontaktes.',
      },
      {
        kind: 'callout',
        tone: 'warning',
        title: 'Ingen må udgå af løbet uden aftale med nødtelefonen',
        text: 'Det betyder, at hverken tropsledere, forældre eller andre henter en spejder undervejs på løbet.',
      },
      {
        kind: 'phone',
        label: 'Forældre-kontakttelefon',
        text: 'Løbets døgnbemandede forældre-kontakttelefon – kun til brug i alvorlige situationer.',
        phone: FORAELDRETELEFON,
        display: FORAELDRETELEFON_DISPLAY,
      },
    ],
  },
  {
    id: 'guidelines',
    title: 'Guidelines',
    summary: 'Forberedelse, pakkeliste og hvad I møder på løbet',
    icon: Backpack,
    blocks: [
      {
        kind: 'text',
        text: 'Nathejk vil for de fleste spejdere være et hårdt løb. Derfor er her lidt guidelines, der kan være med til at gøre det lidt mindre hårdt.',
      },
      {
        kind: 'list',
        lead: 'Inden start er det en god idé, hvis spejderne:',
        items: [
          'Kan finde vej med kort og kompas',
          'Kan klare sig på egen kost (f.eks. med Trangia/stormkøkken el.lign.)',
          'Kan gå cirka 35–50 kilometer med deres udstyr',
          'Kan finde et sovested under åben himmel',
        ],
      },
      {
        kind: 'text',
        text: 'Udover sikkerhedsudstyret og tørklædet, som skal medbringes, er her et forslag til andet udstyr, der kan medbringes.',
      },
      {
        kind: 'list',
        lead: 'Personligt udstyr:',
        items: [
          'Varmt/vindtæt tøj',
          'Uniform',
          'Hue og vanter',
          'Masser af strømper',
          'Undertøj',
          'Vaskegrej/tandbørste',
          'En god lygte og ekstra batterier',
          'Drikkedunk/væske',
          'Fuldt opladet mobiltelefon',
          'Powerbank',
        ],
      },
      {
        kind: 'list',
        lead: 'Patruljeudstyr:',
        items: [
          'Trangia/stormkøkken (husk brændstof og tændstikker)',
          'Mad',
          'Spade + toiletpapir (hvis I skal på “toilettet” i skoven)',
          'Letvægts-presenning/tarp (til at sove under og skjule jer under)',
          'Snor til at bygge bivuak',
          'Affaldsposer',
        ],
      },
      {
        kind: 'callout',
        tone: 'info',
        text: 'Mobiltelefoner er meget praktiske på Nathejk, men de medbringes (ligesom alt andet udstyr) på eget ansvar. Nathejk kan ikke påtage sig at erstatte mobiltelefoner eller andet udstyr, der går i stykker eller mistes under Nathejk.',
      },
      {
        kind: 'text',
        text: 'Patruljen kan frit fordele udstyret mellem medlemmerne, så de stærkeste bærer mest. Der skal bare være det nødvendige sikkerhedsudstyr tilgængeligt for alle medlemmer i patruljen.',
      },
      { kind: 'subheading', text: 'Banditter og guides' },
      {
        kind: 'text',
        text: 'Banditterne kan finde på lidt af hvert. Oftest vil I møde dem på gåben eller cykel, men det kan også hænde, at I møder dem i bil. I kan dog kun blive fanget af banditter, der er til fods. Hvis I bliver fanget af banditterne, skal I fremvise jeres kort og oplyse jeres patruljenummer.',
      },
      {
        kind: 'text',
        text: 'Guides er voksne spejdere, der færdes i løbsområdet for at hjælpe spejderne gennem løbet.',
      },
      { kind: 'subheading', text: 'Skadede og trætte spejdere' },
      {
        kind: 'text',
        text: 'Nathejks procedurer for skadede og trætte spejdere på løbet er fast: Det er altid en samarit, der afgør, om det er forsvarligt at fortsætte.',
      },
      { kind: 'subheading', text: 'Vindere, betaling og forsikring' },
      {
        kind: 'text',
        text: 'Nathejk har ingen vindere eller tabere – og der findes ingen lister over, hvilke patruljer der kom først i mål. Vi hylder dem, der kommer i mål, og dem der kommer igennem uden at blive fanget.',
      },
      {
        kind: 'text',
        text: 'Deltagerbetalingen bliver ikke refunderet ved afbud uanset grund – vi kan have brugt pengene ud fra en forventning om, at I kommer.',
      },
      {
        kind: 'text',
        text: 'Nathejk har ikke ulykkesforsikring på deltagerne. Det er op til deltagerne selv og deres forældre at have styr på det.',
      },
    ],
  },
]

