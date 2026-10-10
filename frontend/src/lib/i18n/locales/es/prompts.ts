import type { Shape } from "../../catalog"
import type { prompts as en } from "../en/prompts"

export const prompts = {
  pullRequest: {
    conflicts:
      "El pull request #{number} ({branch}) tiene conflictos de fusión con {base}. Resuélvelos.",
    checksUnnamed: {
      one: "El CI está fallando en el pull request #{number} ({branch}). {count} comprobación está en rojo; averigua cuál es y corrígela.",
      other:
        "El CI está fallando en el pull request #{number} ({branch}). {count} comprobaciones están en rojo; averigua cuáles son y corrígelas.",
    },
    checksListed:
      "El CI está fallando en el pull request #{number} ({branch}). Corrige estas comprobaciones:\n\n{checks}",
    checksOmitted: {
      one: "\n\n…y {count} comprobación fallida más sin listar.",
      other: "\n\n…y {count} comprobaciones fallidas más sin listar.",
    },
    create:
      "La rama {branch} todavía no tiene pull request. Abre uno con `gh pr create`: antes haz push de la rama si no está en el remoto, escribe el título y la descripción a partir de los commits y del diff contra la rama base, y sigue la plantilla de pull request del repositorio si tiene una.",
  },
  delegate: {
    session: 'Delega en la sesión "{target}": ',
    newWorktree: "Delega en una nueva sesión en worktree: ",
    newWorktreeWithCommand:
      "Delega en una nueva sesión en worktree (ejecuta `lich open --worktree <branch> --prompt <task>`): ",
  },
  reviewComments: {
    header: "Comentarios de revisión:",
  },
  issue: {
    head: "Issue de GitHub #{number}: {title}\n{url}",
  },
} satisfies Shape<typeof en>
