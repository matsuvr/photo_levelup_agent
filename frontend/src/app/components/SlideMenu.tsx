"use client"

import { useEffect, type ReactNode } from "react"

type SlideMenuProps = {
    isOpen: boolean
    onClose: () => void
    children: ReactNode
}

export function SlideMenu({ isOpen, onClose, children }: SlideMenuProps) {
    // Prevent body scroll when menu is open
    useEffect(() => {
        if (isOpen) {
            document.body.style.overflow = "hidden"
        } else {
            document.body.style.overflow = ""
        }
        return () => {
            document.body.style.overflow = ""
        }
    }, [isOpen])

    // Handle escape key
    useEffect(() => {
        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === "Escape" && isOpen) {
                onClose()
            }
        }
        document.addEventListener("keydown", handleEscape)
        return () => document.removeEventListener("keydown", handleEscape)
    }, [isOpen, onClose])

    return (
        <>
            {/* Overlay */}
            <div
                className={`slide-menu-overlay ${isOpen ? "active" : ""}`}
                onClick={onClose}
                aria-hidden="true"
            />

            {/* Menu Panel */}
            <aside
                className={`slide-menu ${isOpen ? "open" : ""}`}
                role="dialog"
                aria-modal="true"
                aria-label="マイページメニュー"
            >
                <div className="slide-menu-header">
                    <h2>マイページ</h2>
                    <button
                        type="button"
                        className="slide-menu-close"
                        onClick={onClose}
                        aria-label="メニューを閉じる"
                    >
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                            <path d="M18 6L6 18M6 6l12 12" />
                        </svg>
                    </button>
                </div>
                <div className="slide-menu-content">
                    {children}
                </div>
            </aside>
        </>
    )
}
